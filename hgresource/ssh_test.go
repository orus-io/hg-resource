package main

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"syscall"
)

func processExists(pid int) bool {
	proc, _ := os.FindProcess(pid)
	err := proc.Signal(syscall.Signal(0))
	// err could also be "permission denied", but for this test it's good enough
	return err == nil
}

var _ = Describe("Ssh", func() {
	Context("When starting ssh-agent", func() {
		vars := "SSH_AGENT_PID=123; export SSH_AGENT_PID;\nFOO=bar; export FOO;\nANSWER=42\n"
		BeforeEach(func() {
			os.Setenv("SSH_AGENT_PID", "")
			os.Setenv("FOO", "")
			os.Setenv("ANSWER", "")
		})

		It("sets the environment variables ssh-agent prints to STDOUT", func() {
			setEnvironmentVariablesFromString(vars)

			Expect(os.Getenv("SSH_AGENT_PID")).To(Equal("123"))
			Expect(os.Getenv("FOO")).To(Equal("bar"))
			Expect(os.Getenv("ANSWER")).To(Equal("42"))
		})

		It("can start and kill the agent", func() {
			err := startSshAgent()
			Expect(err).To(BeNil())

			Expect(os.Getenv("SSH_AGENT_PID")).ToNot(BeEmpty())

			pid, err := strconv.Atoi(os.Getenv("SSH_AGENT_PID"))
			Expect(err).To(BeNil())

			err = killSshAgent()
			Expect(err).To(BeNil())

			Eventually(func() bool {
				return processExists(pid)
			}).Should(BeFalse())
		})
	})

	Context("When creating a temp filename", func() {
		baseFileName := "/tmp/hg-resource-test-base-filename"
		var tempFileName string
		var err error

		BeforeEach(func() {
			tempFileName, err = makeTempFileName(baseFileName)
		})

		It("appends random characters to the base filename", func() {
			Expect(err).To(BeNil())
			Expect(tempFileName).To(HavePrefix(baseFileName + "."))
			Expect(len(tempFileName)).To(BeNumerically(">", len(baseFileName)))
		})
	})

	Context("When asking for the temp directory", func() {
		Context("if $TMPDIR is set", func() {
			BeforeEach(func() {
				os.Setenv("TMPDIR", "/my/tmp/dir")
			})

			It("returns $TMPDIR", func() {
				Expect(getTempDir()).To(Equal("/my/tmp/dir"))
			})

			AfterEach(func() {
				os.Setenv("TMPDIR", "")
			})
		})

		Context("if $TMPDIR is not set", func() {
			BeforeEach(func() {
				os.Setenv("TMPDIR", "")
			})

			It("returns /tmp", func() {
				Expect(getTempDir()).To(Equal("/tmp"))
			})
		})

	})

	Context("When saving the SSH private key and the SSH config file", func() {
		var previousHome string

		BeforeEach(func() {
			previousHome = os.Getenv("HOME")
			os.Setenv("HOME", "/some/home/directory")
		})

		It("can find the users home directory", func() {
			homeDir, err := getHomeDir()

			Expect(err).To(BeNil())
			Expect(homeDir).To(Equal("/some/home/directory"))
		})

		AfterEach(func() {
			os.Setenv("HOME", previousHome)
		})
	})

	Context("When saving a file atomically", func() {
		var dirname string
		var filename string
		content := "hello test runner\n"
		var direrr error
		var err error

		BeforeEach(func() {
			dirname, direrr = makeTempFileName("/tmp/hg-resource-test/test-run")
			filename = path.Join(dirname, "base-filename")
		})

		JustBeforeEach(func() {
			err = atomicSave(filename, []byte(content), 0600, 0707)
		})

		AfterEach(func() {
			os.RemoveAll(dirname)
		})

		Context("if the directory does not exist yet", func() {
			It("creates the directory structure", func() {
				Expect(direrr).To(BeNil())
				Expect(err).To(BeNil())

				dirInfo, statErr := os.Stat(dirname)
				Expect(statErr).To(BeNil())
				Expect(dirInfo.IsDir()).To(BeTrue())
			})

			It("sets the file and directory permissions", func() {
				Expect(direrr).To(BeNil())
				Expect(err).To(BeNil())

				dirInfo, statErr := os.Stat(dirname)
				Expect(statErr).To(BeNil())
				Expect(dirInfo.Mode().Perm()).To(Equal(os.FileMode(0707)))

				fileInfo, statErr := os.Stat(filename)
				Expect(statErr).To(BeNil())
				Expect(fileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))
			})

			It("writes the expected content", func() {
				actualContent, readErr := ioutil.ReadFile(filename)
				Expect(readErr).To(BeNil())
				Expect(string(actualContent)).To(Equal(content))
			})
		})

		Context("if the parent directory already exists", func() {
			var preCreateDirErr error
			BeforeEach(func() {
				preCreateDirErr = os.MkdirAll(dirname, 0770)
				os.Chmod(dirname, 0770)
			})

			It("does not change directory permissions if the directories exist already", func() {
				Expect(preCreateDirErr).To(BeNil())
				Expect(direrr).To(BeNil())

				dirInfo, statErr := os.Stat(dirname)
				Expect(statErr).To(BeNil())
				Expect(dirInfo.Mode().Perm()).To(Equal(os.FileMode(0770)))

				Expect(err).To(BeNil())
			})
		})
	})

})
