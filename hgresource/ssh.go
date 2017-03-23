package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

const (
	tempDirEnv                  = "TMPDIR"
	defaultTempDir              = "/tmp"
	keyFileName                 = "hg-resource-private-key"
	sshAskpassPath              = "/opt/resource/askpass.sh"
	sshClientConfig             = "StrictHostKeyChecking no\nLogLevel quiet\n"
	sshClientConfigFileRelative = ".ssh/config"
)

// Writes SSH private key to a file in $TMPDIR or /tmp, starts ssh-agent and
// loads the key
func loadSshPrivateKey(privateKeyPem string) error {
	tempDir := getTempDir()
	keyFilePath := path.Join(tempDir, keyFileName)

	err := atomicSave(keyFilePath, []byte(privateKeyPem), 0600, 0777)
	if err != nil {
		return fmt.Errorf("Error writing private key to disk: %s", err)
	}

	err = startSshAgent()
	if err != nil {
		return err
	}

	err = addSshKey(keyFilePath)
	if err != nil {
		return err
	}

	homeDir, err := getHomeDir()
	if err != nil {
		return err
	}

	sshClientConfigFile := path.Join(homeDir, sshClientConfigFileRelative)

	err = atomicSave(sshClientConfigFile, []byte(sshClientConfig), 0600, 0700)
	if err != nil {
		return err
	}

	return nil
}

func getHomeDir() (string, error) {
	homeDir := os.Getenv("HOME")
	if len(homeDir) == 0 {
		return "", fmt.Errorf("Unable to retrieve home directory from $HOME")
	}
	return homeDir, nil
}

func addSshKey(keyFilePath string) error {
	stderr := new(bytes.Buffer)
	addCmd := exec.Command("ssh-add", keyFilePath)
	addCmd.Env = append(addCmd.Env, os.Environ()...)
	addCmd.Env = append(addCmd.Env, "DISPLAY=", "SSH_ASKPASS="+sshAskpassPath)
	addCmd.Stderr = stderr
	err := addCmd.Run()
	if err != nil {
		errMsg := stderr.String()
		if len(errMsg) > 0 {
			return fmt.Errorf("Error running ssh-add: %s", errMsg)
		} else {
			return fmt.Errorf("Error running ssh-add: %s", err)
		}
	}

	return nil
}

func startSshAgent() error {
	killSshAgent()

	stdout := new(bytes.Buffer)
	agentCmd := exec.Command("ssh-agent")
	agentCmd.Stdout = stdout

	err := agentCmd.Run()
	if err != nil {
		return fmt.Errorf("Error running ssh-agent: %s", err)
	}

	setEnvironmentVariablesFromString(stdout.String())
	return nil
}

func killSshAgent() error {
	pidString := os.Getenv("SSH_AGENT_PID")
	if len(pidString) == 0 {
		return nil
	}

	pid, err := strconv.Atoi(pidString)
	if err != nil {
		return fmt.Errorf("Error killing ssh-agent: SSH_AGENT_PID not an integer, but: %s", pidString)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	err = proc.Kill()
	if err != nil {
		return err
	}

	return nil
}

func setEnvironmentVariablesFromString(multiLine string) {
	lines := strings.Split(multiLine, "\n")
	for _, line := range lines {
		// we don't support any kind of quoting or escaping
		lineBeforeSemicolon := strings.SplitN(line, ";", 2)
		keyValue := strings.SplitN(lineBeforeSemicolon[0], "=", 2)
		if len(keyValue) == 2 {
			os.Setenv(keyValue[0], keyValue[1])
		}
	}
}

func atomicSave(filename string, content []byte, fileMode os.FileMode, dirMode os.FileMode) error {
	dirName := path.Dir(filename)
	_, pathErr := os.Stat(dirName)
	if pathErr != nil {
		err := os.MkdirAll(dirName, dirMode)
		if err != nil {
			fmt.Errorf("atomicSave(): Error creating directory %s: %s", dirName, err)
		}

		// mkdir syscall typically doesn't set write flags for all/group
		err = os.Chmod(dirName, dirMode)
		if err != nil {
			fmt.Errorf("atomicSave(): %s", err)
		}
	}

	tempFileName, err := makeTempFileName(filename)
	if err != nil {
		return err
	}

	tempFile, err := os.Create(tempFileName)
	if err != nil {
		return fmt.Errorf("atomicSave(): %s", err)
	}

	_, err = tempFile.Write(content)
	if err != nil {
		return fmt.Errorf("atomicSave(): Error writing to file %s: %s", tempFileName, err)
	}

	err = tempFile.Sync()
	if err != nil {
		return err
	}
	err = tempFile.Close()
	if err != nil {
		return err
	}

	err = os.Chmod(tempFileName, fileMode)
	if err != nil {
		return fmt.Errorf("atomicSave(): Error changing permissions of %s: %s", tempFileName, err)
	}

	err = os.Rename(tempFileName, filename)
	if err != nil {
		return fmt.Errorf("Error renaming file %s to %s in atomicSave: %s", tempFileName, filename, err)
	}

	return nil
}

func makeTempFileName(filename string) (string, error) {
	randomBytes := make([]byte, 12)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("Error creating temporary filename: %s", err)
	}

	tempFileName := filename + "." + base32.StdEncoding.EncodeToString(randomBytes)
	return tempFileName, nil
}

func getTempDir() string {
	tempDir := os.Getenv(tempDirEnv)
	if len(tempDir) > 0 {
		return tempDir
	}
	return defaultTempDir
}
