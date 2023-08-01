package git_commands

import (
	"fmt"
)

type BlameCommands struct {
	*GitCommon
}

func NewBlameCommands(gitCommon *GitCommon) *BlameCommands {
	return &BlameCommands{
		GitCommon: gitCommon,
	}
}

func (self *BlameCommands) BlameLineRange(filename string, commit string, firstLine int, numLines int) (string, error) {
	cmdArgs := NewGitCmd("blame").
		Arg("-l").
		Arg(fmt.Sprintf("-L%d,+%d", firstLine, numLines)).
		Arg(commit).
		Arg("--").
		Arg(filename)

	return self.cmd.New(cmdArgs.ToArgv()).RunWithOutput()
}
