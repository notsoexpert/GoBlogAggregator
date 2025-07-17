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

func middlewareLoggedIn(handler func(s *state, cmd command, sqlUser database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		sqlUser, err := s.DBQueries.GetUser(context.Background(), s.Config.CurrentUserName)
		if err != nil {
			return fmt.Errorf("error: current user %s not found - %v", s.Config.CurrentUserName, err)
		}

		return handler(s, cmd, sqlUser)
	}
}

func scrapeFeeds(s *state) error {
	// get next feed
	sqlFeed, err := s.DBQueries.GetNextFeedToFetch(context.Background())
	if err != nil {
		return fmt.Errorf("error: failed to get next feed from database - %v", err)
	}

	// mark as fetched
	err = s.DBQueries.MarkFeedFetched(context.Background(), sqlFeed.ID)

	// fetch the feed
	rssFeed, err := rss.FetchFeed(context.Background(), sqlFeed.Url)
	if err != nil {
		return err
	}

	// iterate and print
	for _, item := range rssFeed.Channel.Item {
		fmt.Println("* ", item.Title)
	}
	return nil
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
	commands.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	commands.register("feeds", handlerFeeds)
	commands.register("follow", middlewareLoggedIn(handlerFollow))
	commands.register("following", middlewareLoggedIn(handlerFollowing))
	commands.register("unfollow", middlewareLoggedIn(handlerUnfollow))



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

	if len(sqlUsers) == 0 {
		fmt.Println("No users registered.")
	}
	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.Args) == 0 {
		return errors.New("error: no request period provided (1s / 1m / 1h / etc)")
	}

	time_between_reqs, err := time.ParseDuration(cmd.Args[0])
	if err != nil {
		return fmt.Errorf("error: failed to parse request period - %v", err)
	}

	fmt.Println("Collecting feeds every ", cmd.Args[0])

	ticker := time.NewTicker(time_between_reqs)
	for ; ; <-ticker.C {
		err = scrapeFeeds(s)
		if err != nil {
			return err
		}
	}

	return nil
}

func handlerAddFeed(s *state, cmd command, sqlUser database.User) error {
	if len(cmd.Args) == 0 {
		return errors.New("error: no name provided")
	}

	if len(cmd.Args) == 1 {
		return errors.New("error: no url provided")
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

	fmt.Printf("Feed \"%s\" has been added.\n", newSqlFeed.Name)

	_, err = s.DBQueries.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID: uuid.NullUUID{UUID: sqlUser.ID, Valid: true},
		FeedID: uuid.NullUUID{UUID: newSqlFeed.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("error: failed to create feed follow entry - %v", err)
	}

	fmt.Printf("%s now following \"%s\"\n", sqlUser.Name, newSqlFeed.Name)

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

	if len(sqlFeeds) == 0 {
		fmt.Println("No feeds pulled to database.")
	}
	return nil
}

func handlerFollow(s *state, cmd command, sqlUser database.User) error {
	if len(cmd.Args) == 0 {
		return errors.New("error: no url provided")
	}

	sqlFeed, err := s.DBQueries.GetFeed(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("error: no feeds added using url %s - %v", cmd.Args[0], err)
	}

	_, err = s.DBQueries.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID: uuid.NullUUID{UUID: sqlUser.ID, Valid: true},
		FeedID: uuid.NullUUID{UUID: sqlFeed.ID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("error: failed to create feed follow entry - %v", err)
	}

	fmt.Printf("%s is now following \"%s\"\n", sqlUser.Name, sqlFeed.Name)
	return nil
}

func handlerFollowing(s *state, cmd command, sqlUser database.User) error {
	sqlFeedFollows, err := s.DBQueries.GetFeedFollowsForUser(context.Background(), uuid.NullUUID{UUID: sqlUser.ID, Valid: true})
	if err != nil {
		return fmt.Errorf("error: failed to retreive follow records for user %s - %v", s.Config.CurrentUserName, err)
	}

	if len(sqlFeedFollows) == 0 {
		fmt.Printf("%s is not currently following any feeds.\n", s.Config.CurrentUserName)
		return nil
	}
	
	fmt.Printf("%s is currently following:\n", s.Config.CurrentUserName)
	for _, follows := range sqlFeedFollows {
		fmt.Printf("* \"%s\"\n", follows.FeedName)
	}

	return nil
}

func handlerUnfollow(s *state, cmd command, sqlUser database.User) error {
	if len(cmd.Args) == 0 {
		return errors.New("error: no url provided")
	}

	err := s.DBQueries.DeleteFeedFollow(context.Background(), database.DeleteFeedFollowParams{
		Name: s.Config.CurrentUserName,
		Url: cmd.Args[0],
	})
	if err != nil {
		return fmt.Errorf("error: failed %s unfollow of %s - %v", s.Config.CurrentUserName, cmd.Args[0], err)
	}

	fmt.Printf("%s has unfollowed feed at %s\n", s.Config.CurrentUserName, cmd.Args[0])
	return nil
}