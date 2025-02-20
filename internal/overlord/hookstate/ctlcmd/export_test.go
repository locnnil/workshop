package ctlcmd

import "fmt"

func AddMockCommand(name string) *MockCommand {
	return addMockCmd(name, false)
}

func addMockCmd(name string, hidden bool) *MockCommand {
	mockCommand := NewMockCommand()
	cmd := addCommand(name, "", "", func() command { return mockCommand })
	cmd.hidden = hidden
	return mockCommand
}

func AddHiddenMockCommand(name string) *MockCommand {
	return addMockCmd(name, true)
}

func RemoveCommand(name string) {
	delete(commands, name)
}

func NewMockCommand() *MockCommand {
	return &MockCommand{
		ExecuteError: false,
	}
}

func (c *MockCommand) Execute(args []string) error {
	c.Args = args

	if c.FakeStdout != "" {
		_, err := c.printf("%s", c.FakeStdout)
		if err != nil {
			return err
		}
	}

	if c.FakeStderr != "" {
		_, err := c.errorf("%s", c.FakeStderr)
		if err != nil {
			return err
		}
	}

	if c.ExecuteError {
		return fmt.Errorf("failed at user request")
	}

	return nil
}

type MockCommand struct {
	baseCommand

	ExecuteError bool
	FakeStdout   string
	FakeStderr   string

	Args []string
}
