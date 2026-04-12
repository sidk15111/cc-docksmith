has to run on linux
code lives in wherever in your system

data of the actual project like the .tars base images cache all lives
in ~/.docksmith 


# what are we building

At its core, Docksmith is a simplified, offline-only clone of Docker. You are building a CLI tool that parses a configuration file (Docksmithfile), builds an image layer by layer, and runs that image in an isolated environment.

You are strictly forbidden from using actual Docker, runc, or any network access during the build/run phases.

The project is divided into three major pillars:

1. The Build Engine & Instructions
You need to write a parser that reads a Docksmithfile and executes exactly six supported instructions:

    FROM: Grabs a pre-downloaded base image from your local ~/.docksmith/ directory.

    COPY: Moves files from your local directory into the image.

    RUN: Executes a command inside the isolated filesystem being built.

    WORKDIR, ENV, CMD: These update the image's configuration metadata but do not create new filesystem layers.

2. Storage, Layers, & The Cache
Every time a COPY or RUN instruction executes, it creates a "layer".

    Layers: A layer is just a .tar archive containing only the files that were changed or added during that step. These are stored using the SHA-256 hash of their contents.

    The Manifest: The final image is represented by a JSON file that lists the configuration and the exact order of the layer hashes needed to assemble the filesystem.

    The Cache: To make builds fast, you must implement a deterministic cache. Before executing a step, Docksmith must generate a hash based on the previous layer, the instruction text, the environment variables, and the working directory. If that hash already exists, you skip the work and use the cached layer.

3. The Container Runtime (The Hard Part)
This is where you execute a process.

    You must extract the .tar layers in order, stacking them in a temporary directory to create a full filesystem.

    You then run the specified CMD inside that temporary directory, fully isolated from the host machine.

    A file written inside this container must not show up on your actual host machine.

# team member roles

## member 1 : parser

parser guy
The build language uses files named Docksmithfile.

It must recognize exactly 6 instructions: FROM, COPY, RUN, WORKDIR, ENV, CMD.

Any unrecognized instruction must fail immediately with a clear error and the exact line number.

updating internal/parser/parser.go
and adding stuff in build case in main.go and in import

to test make dummy Docksmithfile
```
# This is a test Docksmithfile
FROM alpine:latest

WORKDIR /app

COPY src/ dest/
RUN echo "Testing the parser"
ENV DEBUG=true

CMD ["/bin/sh"]
```
works so far!
```
git run cmd/docksmith/main.go build .go
```


## member 2 : fs and manifest (tar guy)

it should be perfectly reproducible, if hashed should give same SHA256 back every single time
regardless of path it just uses /
tarball is how were storing and tar by default adds files in OS order of reading which changes so it would fail
so we will sort alpha and then override timestamp so hash should be the same

editing internal/engine/tar.go

to test
```
mkdir test_context
echo "hello world" > test_context/file1.txt
echo "data" > test_context/file2.txt
```
```
go run cmd/docksmith/main.go test-tar test_context
```
```
sha256sum ~/.docksmith/layers/test_layer.tar
```
do last 2 again after changing timestamp by opening
if they match yay!


