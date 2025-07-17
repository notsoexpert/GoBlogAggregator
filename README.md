# Gator - A Blog Aggregator Written in Go

## Boot.Dev - Create a Blog Aggregator in Go

### Prerequisites:  
    Postgres - [Download](https://www.postgresql.org/download/)  
    Go - [Download](https://go.dev/dl/)  

### Install Instructions:  
1. Clone repository  

`git clone https://github.com/notsoexpert/GoBlogAggregator.git`

2. Navigate to local repository  

`cd goblogaggregator`

3. Install using Go  

`go install`

4. Create a new PostgreSQL called 'gator' using the frontend of your choice  

```
psql

\c gator
```

5. Create a config file in your home directory  
        
`touch ~/.gatorconfig.json`

6. Add the following to your config file:  
        
```
{
    "db_url":"{YOUR_CONNECTION_STRING}?sslmode=disable"
}
```
Where {YOUR_CONNECTION_STRING} is as formatted as follows:  

`protocol://username:password@host:port/database`

You can test this string using your database frontend:  

`psql postgres://postgres:@localhost:5432/gator`

### Usage Instructions:  

Gator is a CLI program that expects at least the name of a command each time it is used.  

Commands:  
        
login  
register  
users  
addfeed  
feeds  
follow  
following  
unfollow  
agg  
browse  
        
1. Users:  

You can register a user using:  

`gator register my_username`

The login command changes users:  

`gator login my_username`

You can list all users with:  

`gator users`

2. Feeds:  

Adding feeds expects a title and a URL:  
    
`gator addfeed "My Blog" https://example.com/myblog

Addfeed automatically follows the feed with the currently logged in user.  

You can list all added feeds:  

`gator feeds`

To follow a feed:  

`gator follow https://example.com/myblog`

This feed must already have been added with addfeed.  

To list all feeds followed the current user:  

`gator following`

You can also unfollow a feed with the current user:  

`gator unfollow https://example.com/myblog`

3. To periodically scrape feeds, the agg command runs continuosly with a timeout parameter:  

`gator agg 10m`

This will scrape feeds every 10 minutes. This will occupy the current context, so you may need to open a new interface to continue running commands.  

4. To list followed posts, with a total post limit:  

`gator browse 10`

    
