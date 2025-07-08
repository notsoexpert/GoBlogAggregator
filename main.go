package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/notsoexpert/goblogaggregator/internal/config"
)

type state struct {
	Config *config.Config
}

type command struct {
	Name string
	Args []string
}

type commands struct {
	Commands map[string]func(*state, command) error
}

func main() {
	var currentState state
	{
		cfg, err := config.Read()
		if err != nil {
			fmt.Println("error reading config: ", err.Error())
			os.Exit(1)
		}
		currentState.Config = &cfg
	}

	var commands commands
	commands.Commands = make(map[string]func(*state, command) error)
	commands.register("login", handlerLogin)

	if len(os.Args) < 2 {
		fmt.Println("error: not enough arguments")
		os.Exit(1)
	}

	var loginCommand command = command{
		Name: os.Args[1],
		Args: os.Args[2:],
	}

	if err := commands.run(&currentState, loginCommand); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.Commands[cmd.Name]
	if !ok {
		return errors.New("error: unregistered command")
	}
	return handler(s, cmd)
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.Commands[name] = f
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.Args) == 0 {
		return errors.New("error: no username provided")
	}

	if err := s.Config.SetUser(cmd.Args[0]); err != nil {
		return err
	}

	fmt.Println("User has been set to", cmd.Args[0])
	return nil
}
