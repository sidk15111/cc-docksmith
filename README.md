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


## member 3: cache guy

if docksmith build is done 2 times without any change, stored layer for 2nd run should be used
not reexecuting everything

for this we make a SHA256 key of the layer done before COPY or RUN so it can be identified without useless ie non changing instructions making difference

editing internal/cache/cache.go

## member 4: runtime and isolation fellow

this is toughest

take a program, lock it inside a single, empty closet, and trick it into believing that the closet is the entire universe.

we have to trick the process into thinking that it has its own private kernel and private fs

we use re-exec pattern

the docksmith binary will configure the Linux namespaces (PID, Mount, UTS) and then execute itself again as a hidden child process. That child process then uses chroot to lock itself inside the extracted temporary directory.

editing internal/runtime/container.go

FOR THIS TO WORK HAS TO BE SUDO
since it is using chroot command

### testing

```
# 1. Create a folder for the fake root filesystem
mkdir alpine-root
cd alpine-root

# 2. Download a tiny Alpine Linux filesystem tarball
wget https://dl-cdn.alpinelinux.org/alpine/v3.18/releases/x86_64/alpine-minirootfs-3.18.4-x86_64.tar.gz

# 3. Extract it (this unpacks bin/, etc/, usr/, var/ into the alpine-root folder)
tar -xzf alpine-minirootfs-3.18.4-x86_64.tar.gz
rm alpine-minirootfs-3.18.4-x86_64.tar.gz
cd ..
```
```
sudo go run cmd/docksmith/main.go test-run ./alpine-root /bin/sh
```
the -E is to pass env var since otherwise it strips all env vars for sec purposes

Your terminal should change to a basic # prompt.

Type ls /. You will only see the Alpine folders, not your host machine's files.

Type env. You will see MOCK_ENV=docksmith_test and none of your actual system environment variables.

Type touch /PROOF.txt.

Type exit to leave the container.

Now, back on your host machine, type ls /. The PROOF.txt file will NOT be there. It will only exist inside ./alpine-root/PROOF.txt. You have successfully isolated a process from the host.

yay it works after tons of debugging

```
debug stuff
go cannot change namespace in middle of running program
for creating a container docksmith has to execute itself
The Parent: The command you actually typed (sudo docksmith test-run).
The Child: The parent secretly spawns a new process calling docksmith child and pushes it into the new namespaces.

parent was initially intentionally wiping childs memory so it booted with fuckall
then it crashed because of unconditional initDocksmithDirs() asking for OS $HOME
added that into a !child check so that it doesnt error out

# ORCHESTRATION

reads the parsed instructions one by one, calculates the cache, runs the container (if needed), packs the tar layer, and finally writes the manifest.json file.

creating build file

internal/engine/build.go

some fuckass amount of errors are coming.
