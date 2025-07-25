// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2018 Canonical Ltd
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

package configcore

import (
	"os"
	"strings"

	"github.com/snapcore/snapd/dirs"
	"github.com/snapcore/snapd/features"
	"github.com/snapcore/snapd/osutil"
	"github.com/snapcore/snapd/sysconfig"
	"github.com/snapcore/snapd/testutil"
)

func init() {
	for _, feature := range features.KnownFeatures() {
		snapName, confName := feature.ConfigOption()
		supportedConfigurations[snapName+"."+confName] = true
	}
}

func earlyExperimentalSettingsFilter(values, early map[string]any) {
	for key, v := range values {
		if strings.HasPrefix(key, "experimental.") && supportedConfigurations["core."+key] {
			early[key] = v
		}
	}
}

func validateExperimentalSettings(tr ConfGetter) error {
	for k := range supportedConfigurations {
		if !strings.HasPrefix(k, "core.experimental.") {
			continue
		}
		if err := validateBoolFlag(tr, strings.TrimPrefix(k, "core.")); err != nil {
			return err
		}
	}
	return nil
}

func doExportExperimentalFlags(_ sysconfig.Device, tr ConfGetter, opts *fsOnlyContext) error {
	var dir string
	if opts != nil {
		dir = dirs.FeaturesDirUnder(opts.RootDir)
	} else {
		dir = dirs.FeaturesDir
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	feat := features.KnownFeatures()
	content := make(map[string]osutil.FileState, len(feat))
	for _, feature := range feat {
		if !feature.IsExported() {
			continue
		}
		isEnabled, err := features.Flag(tr, feature)
		if err != nil {
			return err
		}
		if isEnabled {
			content[feature.String()] = &osutil.MemoryFileState{Mode: 0644}
		}
	}
	_, _, err := osutil.EnsureDirState(dir, "*", content)
	return err
}

func ExportExperimentalFlags(tr ConfGetter) error {
	return doExportExperimentalFlags(nil, tr, nil)
}

// IsSupportedExperimentalFlag checks if passed flag is a supported experimental feature.
func IsSupportedExperimentalFlag(flag string) bool {
	return supportedConfigurations["core.experimental."+flag]
}

// MockSupportedExperimentalFlags mocks list of supported experimental flags for use in testing.
func MockSupportedExperimentalFlags(flags []string) (restore func()) {
	osutil.MustBeTestBinary("MockSupportedExperimentalFlags only can be used in tests")

	restore = testutil.Backup(&supportedConfigurations)
	newConfs := make(map[string]bool, len(flags))
	for _, flag := range flags {
		newConfs["core.experimental."+flag] = true
	}
	supportedConfigurations = newConfs
	return restore
}
