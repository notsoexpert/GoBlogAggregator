package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/notsoexpert/goblogaggregator/internal/config"
	"github.com/notsoexpert/goblogaggregator/internal/database"
	"github.com/notsoexpert/goblogaggregator/internal/rss"
)

type state struct {
	Config    *config.Config
	DBQueries *database.Queries
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

	db, err := sql.Open("postgres", currentState.Config.DBUrl)
	if err != nil {
		fmt.Println("error: failed to connect to database")
		os.Exit(1)
	}

	currentState.DBQueries = database.New(db)

	var commands commands
	commands.Commands = make(map[string]func(*state, command) error)
	commands.register("login", handlerLogin)
	commands.register("register", handlerRegister)
	commands.register("reset", handlerReset)
	commands.register("users", handlerUsers)
	commands.register("agg", handlerAgg)
	commands.register("addfeed", handlerAddFeed)
	commands.register("feeds", handlerFeeds)

	if len(os.Args) < 2 {
		fmt.Println("error: not enough arguments")
		os.Exit(1)
	}

	var inputCommmand command = command{
		Name: os.Args[1],
		Args: os.Args[2:],
	}

	if err := commands.run(&currentState, inputCommmand); err != nil {
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

	sqlUser, err := s.DBQueries.GetUser(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("error: user not found - %v", err)
	}

	if err := s.Config.SetUser(sqlUser.Name); err != nil {
		return fmt.Errorf("error: failed to set user - %v", err)
	}

	fmt.Println("User has been set to", sqlUser.Name)
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.Args) == 0 {
		return errors.New("error: no username provided")
	}

	sqlUser, err := s.DBQueries.CreateUser(context.Background(), database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.Args[0],
	})
	if err != nil {
		return fmt.Errorf("error: failed to create user in database - %v", err)
	}

	if err := s.Config.SetUser(cmd.Args[0]); err != nil {
		return fmt.Errorf("error: failed to switch to new user - %v", err)
	}
	fmt.Printf("User %s created successfully! Current user is now %s.\n", cmd.Args[0], s.Config.CurrentUserName)

	fmt.Printf("User data: %v\n", sqlUser)

	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.DBQueries.ResetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error: failed to reset users - %v", err)
	}

	fmt.Println("User data has been reset.")
	return nil
}

func handlerUsers(s *state, cmd command) error {
	sqlUsers, err := s.DBQueries.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error: failed to retrieve users - %v", err)
	}

	for _, user := range sqlUsers {
		fmt.Print("* ", user.Name)
		if strings.Contains(s.Config.CurrentUserName, user.Name) {
			fmt.Print(" (current)")
		}
		fmt.Println()
	}
	return nil
}

func handlerAgg(s *state, cmd command) error {
	rssFeed, err := rss.FetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return err
	}

	fmt.Printf("RSSFeed from https://www.wagslane.dev/index.xml:\n%v\n", rssFeed)
	return nil
}

func handlerAddFeed(s *state, cmd command) error {
	if len(cmd.Args) == 0 {
		return errors.New("error: no name provided")
	}

	if len(cmd.Args) == 1 {
		return errors.New("error: no url provided")
	}

	sqlUser, err := s.DBQueries.GetUser(context.Background(), s.Config.CurrentUserName)
	if err != nil {
		return fmt.Errorf("error: current user %s not found - %v", s.Config.CurrentUserName, err)
	}

	newSqlFeed, err := s.DBQueries.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      cmd.Args[0],
		Url:       cmd.Args[1],
		UserID:    uuid.NullUUID{UUID: sqlUser.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("error: failed to create feed in database - %v", err)
	}

	fmt.Printf("Feed %s has been added.\n", newSqlFeed.Name)

	fmt.Printf("Feed data:\n%v\n", newSqlFeed)
	return nil
}

func handlerFeeds(s *state, cmd command) error {
	sqlFeeds, err := s.DBQueries.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("error: failed to retrieve feeds - %v", err)
	}

	for _, feed := range sqlFeeds {
		sqlUser, err := s.DBQueries.GetUserByID(context.Background(), feed.UserID.UUID)
		if err != nil {
			return fmt.Errorf("error: failed to retrieve user from user_id %d referenced by feed %d - %v", feed.UserID.UUID, feed.ID, err)
		}
		fmt.Printf(`
		Name: "%s"
		URL: %s
		Creator: %s
		`, feed.Name, feed.Url, sqlUser.Name)
		fmt.Println()
	}
	return nil
}
