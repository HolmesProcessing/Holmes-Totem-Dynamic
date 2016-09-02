# Holmes-Totem-Dynamic: A Holmes Processing Investigation Planner for Time Intensive Large-scale Analysis

Contributions are always welcome and appreciated! If you would like to contribute please read the [official document](http://holmes-processing.readthedocs.io/en/latest/) and use the information in this CONTRIBUTING.md file as a guide. If you have questions or are unsure about something you would like to implement, please open a new issue. We would be happy to discuss the idea with you.

## Services
New Services are simple to create and are always much appreciated. When implementing a Service you will need to provide:

1. A RESTful interface for Totem-Dynamic to interact with.
2. Add service information to Totem-Dynamic's configuration file.

Additionally, please keep in mind that Totem-Dynamic is optimized for executing long running tasks. For example, this Planner is perfect for Dynamic analysis and querying 3rd party services that take a long time to execute. When needing to rapidly perform quick tasks, such as static analysis, please consider making a [Totem Service](https://github.com/HolmesProcessing/Holmes-Totem) instead.

### Core Components
#### RESTful Endpoints
The following endpoints are the standard and expected endpoints for totem and totem-dynamic:

| Endpoint | Operation | System |
| --- | --- | --- |
| `/` | provide information about the service | Totem and Totem-Dynamic |
| `/analyze/?obj=` | perform tasking and return results | Totem |
| `/feed/?obj=` | submit tasking to the service | Totem-Dynamic |
| `/check/?taskid=` | check to see if the tasking is complete | Totem-Dynamic |
| `/results/?taskid=` | receive service results | Totem-Dynamic |
| `/status/` | retrieve status | Totem-Dynamic |

#### Docker
We uses Docker and Docker-Compose to manage services. This provides a few nice benefits: keeps most issues from replicating, allowing for easier restart, easier status information, etc. However, to manage the overhead we request that the following DockerFile templates are used. This is because it speeds up the container build time and reduces the on-disk size.

For Go:
```dockerfile
FROM golang:alpine

# create folder
RUN mkdir -p /service
WORKDIR /service

# get go dependencies
RUN apk add --no-cache \
		git \
	&& go get github.com/julienschmidt/httprouter \
	&& rm -rf /var/cache/apk/*

###
# [Service] specific options
###
...
```

For Python:
```dockerfile
FROM python:alpine

# add tornado
RUN pip3 install tornado

# create folder
RUN mkdir -p /service
WORKDIR /service

# add holmeslibrary
RUN apk add --no-cache \
		wget \
	&& wget https://github.com/HolmesProcessing/Holmes-Totem-Service-Library/archive/v0.1.tar.gz \
	&& tar xf v0.1.tar.gz \
	&& mv Holmes-Totem-Service* holmeslibrary \
	&& rm -rf /var/cache/apk/* v0.1.tar.gz

###
# [Service] specific options
###
...
```

### Configuration

#### Configuration File
The Service configuration file should be written in JSON and named `service.conf.example`. 

Totem-Dynamic should be configured. Details on how to do this can be found in the official [documentation](http://holmes-processing.readthedocs.io/en/latest/).

#### Port Selection
Internal ports should always be `8080` when using Docker. 

External ports should be listed alphabetically starting with the following range:

| Range | Service Type |
| --- | --- |
| 97xx | No File |
| 72xx | File Based |

### Code Standards
Services can be written in any language. However, we recommend using Go (with [httprouter](https://godoc.org/github.com/julienschmidt/httprouter)) or Python (with [Tornado](http://www.tornadoweb.org/en/stable/)) for the entire Service or at least the interface. The example Docker Files will provide both packages.

#### Standard Libraries
The [Holmes Processing standard library](https://github.com/HolmesProcessing/Holmes-Totem-Service-Library) should be used when appropriate. It provides helpful functions for go and python.

#### Language Style
The code base of a Service should conform to the recommended style for the programming language. 

| Language | Style Documentation | Checking Tool |
| --- | --- | --- |
| Go | [Effective Go](https://golang.org/doc/effective_go.html) | `go fmt` |
| Python | [PEP 9](http://pep8.org/) | [pycodestyle](https://github.com/PyCQA/pycodestyle) |

### Output
The Service should return two outputs: HTTP codes and the results of the Service.

#### HTTP Error Codes
*work in progress*

#### Results
Results should be returned as sane JSON. All care should be given to return the data in a native format. For example, a Service for VirusTotal should return the original response by VirusTotal as it is already sane JSON. However, in cases when modification is needed, please provide an example in the README.md file. 
