# GO Friends List

This is a simple friends list notification application supporting either TCP and UDP protocols, used to demonstrate how we can communicate between several connections that can be used to broadcast messages through channels.

- It notifies the active friends when connecting
- It nofities the active friends when disconnecting

PS: To keep it simple no validations have been added for example to detect if the same user has "logged" more than once or if the friends list has any duplicated user id, of if they are mutual friends to receive the notification for example. we are assuming the best case scenario here to show the communication working between them.

- Both server and client use the port 8080 as default, but can be changed if adding the flag -port when running it.
- Both server and client use the protocol tcp as default, but can be changed if adding the flag -protocol when running it.

## Getting Set Up

Before running the application, you will need to ensure that you have a few requirements installed;
You will need Go.

### Go

[Go](https://golang.org/) is an open source programming language that makes it easy to build simple, reliable, and efficient software.

## Running the server

    go run cmd/server/main.go


Usage of server:

    -port int
        Port. (default 8080)
    -protocol string
            Protocol used, currently supporting tcp and udp. (default "tcp")
    

## Running the client

We can run mulitple instances of the client and the flag -p must be provided to indicate the payload sent with the user identification and his friends list.
    
    go run cmd/client/main.go -payload '{\"user_id\": 1, \"friends\": [2,3,4]}'

Usage of client:

    -payload string
        User Identification. (default "{\"user_id\": 0, \"friends\": []}")
    -port int
            TCP Port. (default 8080)
    -protocol string
            Protocol used, currently supporting tcp and udp. (default "tcp")

## TODO

- add chat capability
- improve UDP heartbeat and the client

[@maxalencar](https://github.com/maxalencar)