// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package skills

import (
	"errors"
	"sort"
	"sync"
)

// Repository stores all known snappy skills and slots and types.
type Repository struct {
	// Protects the internals from concurrent access.
	m      sync.Mutex
	types  []Type
	skills []*Skill
}

var (
	// ErrDuplicate is reported when type, skill or slot already exist.
	ErrDuplicate = errors.New("duplicate found")
	// ErrSkillNotFound is reported when skill cannot be looked up.
	ErrSkillNotFound = errors.New("skill not found")
	// ErrTypeNotFound is reported when skill type cannot found.
	ErrTypeNotFound = errors.New("skill type not found")
)

// NewRepository creates an empty skill repository.
func NewRepository() *Repository {
	return &Repository{}
}

// AllTypes returns all skill types.
func (r *Repository) AllTypes() []Type {
	r.m.Lock()
	defer r.m.Unlock()

	types := make([]Type, len(r.types))
	copy(types, r.types)
	return types
}

// Type returns the type with a given name.
func (r *Repository) Type(typeName string) Type {
	r.m.Lock()
	defer r.m.Unlock()

	return r.unlockedType(typeName)
}

// AddType adds a skill type to the repository.
// NOTE: API exception, Type is an interface, so it cannot use simple types as arguments.
func (r *Repository) AddType(t Type) error {
	r.m.Lock()
	defer r.m.Unlock()

	typeName := t.Name()
	if err := ValidateName(typeName); err != nil {
		return err
	}
	if i, found := r.unlockedTypeIndex(typeName); !found {
		r.types = append(r.types[:i], append([]Type{t}, r.types[i:]...)...)
		return nil
	}
	return ErrDuplicate
}

// AllSkills returns all skills of the given type.
// If skillType is the empty string, all skills are returned.
func (r *Repository) AllSkills(skillType string) []*Skill {
	r.m.Lock()
	defer r.m.Unlock()

	var result []*Skill
	if skillType == "" {
		result = make([]*Skill, len(r.skills))
		copy(result, r.skills)
	} else {
		result = make([]*Skill, 0)
		for _, skill := range r.skills {
			if skill.Type == skillType {
				result = append(result, skill)
			}
		}
	}
	return result
}

// Skills returns the skills offered by the named snap.
func (r *Repository) Skills(snapName string) []*Skill {
	r.m.Lock()
	defer r.m.Unlock()

	var result []*Skill
	for _, skill := range r.skills {
		if skill.Snap == snapName {
			result = append(result, skill)
		}
	}
	return result
}

// Skill returns the specified skill from the named snap.
func (r *Repository) Skill(snapName, skillName string) *Skill {
	r.m.Lock()
	defer r.m.Unlock()

	return r.unlockedSkill(snapName, skillName)
}

// AddSkill adds a skill to the repository.
// Skill names must be valid snap names, as defined by ValidateName.
// Skill name must be unique within a particular snap.
func (r *Repository) AddSkill(snapName, skillName, typeName, label string, attrs map[string]interface{}) error {
	r.m.Lock()
	defer r.m.Unlock()

	// Reject skill with invalid names
	if err := ValidateName(snapName); err != nil {
		return err
	}
	if err := ValidateName(skillName); err != nil {
		return err
	}
	// TODO: ensure that given snap really exists

	t := r.unlockedType(typeName)
	if t == nil {
		return ErrTypeNotFound
	}
	if r.unlockedSkill(snapName, skillName) != nil {
		return ErrDuplicate
	}
	skill := &Skill{
		Name:  skillName,
		Snap:  snapName,
		Type:  typeName,
		Attrs: attrs,
		Label: label,
	}
	// Reject skill that don't pass type-specific sanitization
	if err := t.Sanitize(skill); err != nil {
		return err
	}
	r.skills = append(r.skills, skill)
	sort.Sort(bySkillSnapAndName(r.skills))
	return nil
}

// RemoveSkill removes the named skill provided by a given snap.
// Removing a skill that doesn't exist returns a ErrSkillNotFound.
// Removing a skill that is granted returns ErrSkillBusy.
func (r *Repository) RemoveSkill(snapName, skillName string) error {
	r.m.Lock()
	defer r.m.Unlock()

	// TODO: Ensure that the skill is not used anywhere
	for i, skill := range r.skills {
		if skill.Snap == snapName && skill.Name == skillName {
			r.skills = append(r.skills[:i], r.skills[i+1:]...)
			return nil
		}
	}
	return ErrSkillNotFound
}

// Private unlocked APIs

func (r *Repository) unlockedType(typeName string) Type {
	if i, found := r.unlockedTypeIndex(typeName); found {
		return r.types[i]
	}
	return nil
}

func (r *Repository) unlockedTypeIndex(typeName string) (int, bool) {
	// Assumption: r.types is sorted
	i := sort.Search(len(r.types), func(i int) bool { return r.types[i].Name() >= typeName })
	if i < len(r.types) && r.types[i].Name() == typeName {
		return i, true
	}
	return i, false
}

func (r *Repository) unlockedSkill(snapName, skillName string) *Skill {
	// Assumption: r.skills is sorted
	i := sort.Search(len(r.skills), func(i int) bool {
		if r.skills[i].Snap != snapName {
			return r.skills[i].Snap >= snapName
		}
		return r.skills[i].Name >= skillName
	})
	if i < len(r.skills) && r.skills[i].Snap == snapName && r.skills[i].Name == skillName {
		return r.skills[i]
	}
	return nil
}

// Support for sort.Interface

type bySkillSnapAndName []*Skill

func (c bySkillSnapAndName) Len() int      { return len(c) }
func (c bySkillSnapAndName) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c bySkillSnapAndName) Less(i, j int) bool {
	if c[i].Snap != c[j].Snap {
		return c[i].Snap < c[j].Snap
	}
	return c[i].Name < c[j].Name
}
