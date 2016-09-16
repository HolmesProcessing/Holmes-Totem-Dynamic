# Holmes-Totem-Dynamic: An Investigation Planner for managing long running services.

## Overview

Just like Holmes-Totem the "Dynamic" Planner is responsible for turning data into information by performing feature extraction against submitted objects. When tasked, Holmes-Totem-Dynamic schedules the execution of its services which are focused on dynamic and other long or indefinite running analysis tasks. 


## Highlights

### Inner workings

Totem-Dynamic is based on distributed_cuckoo and uses a similar concept. Just like Totem it accepts incoming tasks via AMQP and sends the results back via AMQP as soon as they are availible.
Internally the application is mostly devided in three parts. `feed` is accepting new tasks from the outside, checks the status of the requested service and - if everything is fine - submits the task to the service or else postpones it until the service status is good again.  `check` then periodically checks if the service is done with the task or if it still need time or if an error occured. `submit` then collects the results from the service and sends it out 

![Diagramm of Totem-Dynamic](https://i.imgur.com/WmZzxzF.png)

###

| Name               | Object Type           | Description     |
| ------------------ | --------------------- | --------------- |
| cuckoo             | Binary Executable     | Performs a cuckoo analysis on the given binaries
| VirusTotal **PLANNED**         | Binary Executable, IPv4/6, Domain | Returns available [VirusTotal](https://www.virustotal.com/) information with a public or private key

## Dependencies

The only needed dependency is a AMQP library for Go. If you setup your [Go environment](https://golang.org/doc/install) correctly you can simple do a

    go get -u github.com/streadway/amqp

and you are done.


## Compilation

After cloning this project you can build it using `go build` or `go install`. This requires a configured and working [Go environment](https://golang.org/doc/install).


## Installation

You can deploy the binary everywhere you want, per default it is statically linked and does not need any libraries on the machine.

The only thing you need to keep in mind is that the Docker containers hosting the services need to be running on the same machine. To build them, setup Docker on the machine, decent into the `./services/` folder, choose the services you would like to run and forward Port 8080 from the container. Don't forget to also configure the `service.conf` file in each folder before building the container.

Next up you need to fill the `config/totem-dynamic.conf.example` with your own values (and services you'd like to run) and rename it to `totem-dynamic.conf`.

After this simply execute the compiled binary and add tasks to the amqp input queue defined in your configuration file.
