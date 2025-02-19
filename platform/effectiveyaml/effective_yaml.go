/*
 * Copyright 2021-2024 JetBrains s.r.o.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package effectiveyaml

import (
	"errors"
	"github.com/JetBrains/qodana-cli/v2024/platform/msg"
	"github.com/JetBrains/qodana-cli/v2024/platform/qdyaml"
	"github.com/JetBrains/qodana-cli/v2024/platform/utils"
	"github.com/JetBrains/qodana-cli/v2024/tooling"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

type Data struct {
	ConfigDir               string
	EffectiveQodanaYamlPath string
	LocalQodanaYamlPath     string
	QodanaConfigJsonPath    string
	EffectiveQodanaYaml     qdyaml.QodanaYaml
}

func BuildEffectiveYaml(
	projectDir string,
	localQodanaYamlPathFromRoot string,
	globalConfigurationsFile string,
	globalConfigId string,
	jrePath string,
	systemDir string,
	effectiveConfigDirName string,
	logDir string,
) (Data, error) {
	localQodanaYamlPathFromRoot = qdyaml.FindDefaultLocalQodanaYaml(projectDir)

	configLoaderCli := createConfigLoaderCliJar(systemDir)
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			log.Warnf("Failed to delete config-loader-cli.jar: %s", err)
		}
	}(configLoaderCli)

	effectiveConfigDir := filepath.Join(systemDir, effectiveConfigDirName)

	localQodanaYamlFullPath := qdyaml.GetLocalNotEffectiveQodanaYamlPathWithProject(
		projectDir,
		localQodanaYamlPathFromRoot,
	)
	args := configurationLoaderCliArgs(
		jrePath,
		configLoaderCli,
		localQodanaYamlFullPath,
		globalConfigurationsFile,
		globalConfigId,
		effectiveConfigDir,
	)
	log.Debugf("Creating effective configuration in '%s' directory, args: %v", effectiveConfigDir, args)
	if _, _, res, err := utils.LaunchAndLog(logDir, "config-loader-cli", args...); res > 0 || err != nil {
		os.Exit(res)
	}

	effectiveQodanaYamlData := getEffectiveQodanaYamlData(effectiveConfigDir)
	err := verifyEffectiveQodanaYamlIdeAndLinterMatchLocal(effectiveQodanaYamlData, localQodanaYamlPathFromRoot)
	if err != nil {
		return effectiveQodanaYamlData, err
	}
	msg.SuccessMessage("Loaded Qodana Configuration")
	return effectiveQodanaYamlData, nil
}

func createConfigLoaderCliJar(systemDir string) string {
	configLoaderCliJarPath := filepath.Join(systemDir, "tools", "config-loader-cli.jar")
	if isFileExists(configLoaderCliJarPath) {
		err := os.Remove(configLoaderCliJarPath)
		if err != nil {
			log.Fatalf("Failed to delete existing config-loader-cli.jar: %s", err)
		}
	}
	err := os.MkdirAll(filepath.Dir(configLoaderCliJarPath), 0755)
	if err != nil {
		log.Fatalf("Failed to create directory for config-loader-cli.jar: %s", err)
	}
	log.Debugf("creating config-loader-cli.jar at '%s'", configLoaderCliJarPath)
	err = os.WriteFile(configLoaderCliJarPath, tooling.ConfigLoaderCli, 0644)
	if err != nil {
		log.Fatalf("Failed to write config-loader-cli.jar content to %s: %s", configLoaderCliJarPath, err)
	}
	return configLoaderCliJarPath
}

func configurationLoaderCliArgs(
	jrePath string,
	configLoaderCliJarPath string,
	localQodanaYamlPath string,
	globalConfigurationsFile string,
	globalConfigId string,
	effectiveConfigDir string,
) []string {
	if jrePath == "" {
		log.Fatal("JRE not found. Required for effective configuration creation in native analysis.")
	}
	if configLoaderCliJarPath == "" {
		log.Fatal("config-loader-cli.jar not found. Required for effective configuration creation in native analysis.")
	}

	var err error
	args := []string{
		utils.QuoteIfSpace(utils.QuoteForWindows(jrePath)),
		"-jar",
		utils.QuoteForWindows(configLoaderCliJarPath),
	}

	effectiveConfigDirAbs, err := filepath.Abs(effectiveConfigDir)
	if err != nil {
		log.Fatalf(
			"Failed to compute absolute path of effective configuration directory %s: %s",
			effectiveConfigDir,
			err,
		)
	}
	args = append(args, "--effective-config-out-dir", utils.QuoteForWindows(effectiveConfigDirAbs))

	if isFileExists(localQodanaYamlPath) {
		localQodanaYamlPathAbs, err := filepath.Abs(localQodanaYamlPath)
		if err != nil {
			log.Fatalf(
				"Failed to compute absolute path of local qodana.yaml file %s: %s",
				localQodanaYamlPath,
				err,
			)
		}
		args = append(args, "--local-qodana-yaml", utils.QuoteForWindows(localQodanaYamlPathAbs))
	}

	if globalConfigurationsFile != "" {
		globalConfigurationsFileAbs, err := filepath.Abs(globalConfigurationsFile)
		if err != nil {
			log.Fatalf(
				"Failed to compute absolute path of global configurations file %s: %s",
				globalConfigurationsFile,
				err,
			)
		}
		args = append(args, "--global-configs-file", utils.QuoteForWindows(globalConfigurationsFileAbs))
	}
	if globalConfigId != "" {
		args = append(args, "--global-config-id", utils.QuoteForWindows(globalConfigId))
	}
	return args
}

func getEffectiveQodanaYamlData(effectiveConfigDir string) Data {
	effectiveQodanaYamlPath := filepath.Join(effectiveConfigDir, "effective.qodana.yaml")
	if !isFileExists(effectiveQodanaYamlPath) {
		effectiveQodanaYamlPath = ""
	}
	localQodanaYamlPath := filepath.Join(effectiveConfigDir, "qodana.yaml")
	if !isFileExists(localQodanaYamlPath) {
		localQodanaYamlPath = ""
	}
	qodanaConfigJsonPath := filepath.Join(effectiveConfigDir, "qodana-config.json")
	if !isFileExists(qodanaConfigJsonPath) {
		qodanaConfigJsonPath = ""
	}

	if effectiveQodanaYamlPath != "" && qodanaConfigJsonPath == "" {
		log.Fatal("effective.qodana.yaml file doesn't have a qodana-config.json file.")
	}
	if localQodanaYamlPath != "" && effectiveQodanaYamlPath == "" {
		log.Fatal("Local qodana.yaml file doesn't have an effective.qodana.yaml file.")
	}

	effectiveQodanaYaml := qdyaml.LoadQodanaYamlByFullPath(effectiveQodanaYamlPath)
	return Data{
		ConfigDir:               effectiveConfigDir,
		EffectiveQodanaYamlPath: effectiveQodanaYamlPath,
		LocalQodanaYamlPath:     localQodanaYamlPath,
		EffectiveQodanaYaml:     effectiveQodanaYaml,
	}
}

func isFileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		log.Fatalf("Failed to verify existence of file %s: %s", path, err)
	}
	return false
}

func verifyEffectiveQodanaYamlIdeAndLinterMatchLocal(
	effectiveQodanaYamlData Data,
	localQodanaYamlPathFromRoot string,
) error {
	effectiveLinter := effectiveQodanaYamlData.EffectiveQodanaYaml.Linter
	effectiveIde := effectiveQodanaYamlData.EffectiveQodanaYaml.Ide
	if effectiveLinter == "" && effectiveIde == "" {
		return nil
	}

	isLocalQodanaYamlPresent := effectiveQodanaYamlData.LocalQodanaYamlPath != ""
	if isLocalQodanaYamlPresent {
		localQodanaYaml := qdyaml.LoadQodanaYamlByFullPath(effectiveQodanaYamlData.LocalQodanaYamlPath)

		topMessageTemplate := "'%s: %s' is specified in one of files provided by 'imports' from " + localQodanaYamlPathFromRoot + " '%s' is required in root qodana.yaml"
		bottomMessageTemplate := "Add `ide: %s` to " + localQodanaYamlPathFromRoot
		if effectiveIde != localQodanaYaml.Ide {
			msg.ErrorMessage(topMessageTemplate, "ide", effectiveIde, "ide")
			msg.ErrorMessage(bottomMessageTemplate, effectiveIde)
			return errors.New("effective.qodana.yaml `ide` doesn't match root qodana.yaml `ide`")
		}
		//goland:noinspection GoDfaConstantCondition
		if effectiveLinter != localQodanaYaml.Linter {
			msg.ErrorMessage(topMessageTemplate, "linter", effectiveLinter, "linter")
			msg.ErrorMessage(bottomMessageTemplate, effectiveLinter)
			return errors.New("effective.qodana.yaml `linter` doesn't match root qodana.yaml `linter`")
		}
	}
	return nil
}
