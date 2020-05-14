# Go Nyzo
Nyzo verifier written in Go.

## Pragmatic Part
This is an implementation of a Nyzo verifier written in Golang. 
The reference implementation in Java can be found at https://github.com/n-y-z-o/nyzoVerifier. 
The implemented version (at time of writing) is 580.

The MVP of this project is not a full verifier, it is an observer/archive node that is capable of 
assembling the entire past blockchain, verify it, follow its growth and store blocks, as well as transactions, cycle events
and node status information in a database.

### Installation
IMPORTANT: this experimental node accesses the same resources that the Java node would, so do not run it on a system 
with an existing Java node. See below for an installation example on Ubuntu 18.04 LTS.

1. Download and install the latest version of Go
2. Install the project's sole dependency: `go get -u github.com/go-sql-driver/mysql`
3. Clone this repo into the correct location in your go path, typically ~/go/src/github.com/cryptic-monk/go-nyzo
4. Set up /var/lib/nyzo/production (see below)
5. Run the archive node with `go build github.com/cryptic-monk/go-nyzo/cmd/archive_node &&  go run github.com/cryptic-monk/go-nyzo/cmd/archive_node`

#### Configuration in /var/lib/nyzo/production
The Go node requires similar information in the above directory to work properly. Please create that directory and add
the following files:

managed_verifiers, nickname, preferences, trusted_entry_points

`managed_verifiers`
The archive node needs at least one managed verifier that tracks the blockchain. This can be an in 
cycle node, or it can be an out of cycle node with always_track_blockchain activated. Furthermore, this node should have enough
data to fill the gap between the online block repo and the current frozen edge. Typically, this means that the blockchain tracking
node must have been operative for about 24 hours to allow the archive node to catch up from the online repo to the frozen edge.

`nickname`
Works exactly like with Java, can also be left away.

`preferences`
The archive node requires access to a MySQL database, e.g.:

    sql_protocol=tcp
    sql_host=127.0.0.1
    sql_port=3306
    sql_db_name=nyzo
    sql_user=nyzo
    sql_password=somepassword
    
To speed up initial chain loading while accepting a negligible reduction of security, 
set `fast_chain_initialization=1` in preferences. At the time of writing, the archive node takes roughly 12h
to reconstruct the entire chain, save it to a database, and catch up to the frozen edge on a machine with 4 dedicated CPUS and 8GB of RAM. 
DB size is roughly 30GB. Use an SSD with XFS for the DB if possible, and set/change the following variables in mysql.conf:

    innodb_flush_log_at_trx_commit = 2
    innodb_flush_method=O_DIRECT
    innodb_log_file_size = 128M
    
I spent a lot of time optimizing the archive node, but given its purpose, it has to crunch quite a bit more data
than a regular verifier or sentinel, you'll feel better running it on a machine with at least 2 dedicated CPUs and 4GB 
of RAM, double that would be even better.

`trusted_entry_points`
Same as for Java, just copy it over from there.

The archive node can be reset by deleting all database entries in the database down to a certain height. Apart from the cycle events table, there should be no other tables
that need to be touched. The cycle events should be deleted down to the same height that the blocks were deleted down to. 

The node's database follows the Nyzo Open DB structure, see here: https://github.com/Open-Nyzo/Open-Nyzo-Projects/tree/master/Open-DB.

#### Installation Example: Ubuntu 18.04 LTS

Install Go:

    cd
    wget https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.14.2.linux-amd64.tar.gz

Set path:

- open ~/.profile in an editor, e.g. `nano ~/.profile`
- add the following to the end of the file: `export PATH=$PATH:/usr/local/go/bin`
- exit the editor
- `source ~/.profile`

Check:

    go version

=> This should spit out 1.14.2, the version you installed above. If not, debug your Go setup.
    
Install dependency:

    go get -u github.com/go-sql-driver/mysql
    
Get go-nyzo repo:

    cd ~/go/src/github.com
    mkdir cryptic-monk
    cd cryptic-monk
    git clone https://github.com/cryptic-monk/go-nyzo.git
    
Run:

    go build github.com/cryptic-monk/go-nyzo/cmd/archive_node &&  go run github.com/cryptic-monk/go-nyzo/cmd/archive_node
    
Run the node in tmux or similar so that you can later log out of your session. Stop the node with Ctrl+C.

Update:

    cd ~/go/src/github.com/cryptic-monk/go-nyzo
    git pull
    go build github.com/cryptic-monk/go-nyzo/cmd/archive_node &&  go run github.com/cryptic-monk/go-nyzo/cmd/archive_node

### Project Structure

#### Directories
- `cmd`: Main programs, essentially what Java knows as "run modes". Only `archive_node` is fully operational.
- `pkg`: Packages that could potentially also be used in other Go programs, only one for now, `identity`.
- `internal`: Packages that are likely only of relevance to this implementation of a Nyzo verifier.
- `test`: Testing related data and code.

#### Core Architecture
By its nature, a blockchain node features of a number of often singleton-like modules with intense interdependence. 
A Nyzo specific example - a similar one could be made for any other blockchain node project:

- When a new **block** is frozen, the **balance list** cannot be updated without the node knowing **the cycle**, because without
knowing the cycle, no decision can be made whether any **cycle signatures** and **cycle transactions** in the block are valid
and/or have been accepted
- The **node manager** needs to know **the cycle** to determine how many resources it should dedicate to an individual node
that it is in contact with
- The **blockchain handler** cannot freeze a new block without knowing its **balance list** and **the cycle**
- **the cycle** can only be calculated based on information about actual blocks frozen by the **blockchain handler**

Furthermore, we might want to use different types of these often singleton-like modules depending on a particular node's needs, e.g.
some kind of light node without an intention to join the Nyzo cycle might want to run a trimmed-down version of the node manager.

To maintain flexibility in the light of these intense interdependencies, we use two strategies: **dependency injection** of the core
blockchain modules (accessed via interfaces), and **message passing**. 

Dependency injection (direct function calling) happens when the consumption of the data needs to be closely linked to its production, e.g. 
the above signature verification cannot continue without knowing whether the signing verifier is in cycle, so the determination of that
fact is done via a function call.

Message passing happens when the processing of once produced data can be somewhat independent of its production, e.g. the fact that
a new block has been frozen is emitted as a message for, say, the database storage module to pick up, disassemble and store in the database.

##### Dependency Injection
The node first builds a blockchain context (nyzo/interfaces/Context) by setting up the type of components
it needs, e.g. a blockchain manager, a block file manager, a balance manager, a node manager. Those elements must all satisfy the
interface definitions given in nyzo/interfaces, and via the context built by the node they all know about each other and can
talk to each other via function calls. At the same time, the interfaces remain very clearly demarcated. 

A blockchain context is initialized on startup by calling Initialize() on each component, then the messaging loop of each component
is kicked off by calling its Start() function. 

##### Message Passing
Two types of messages flow through the system via Go's "channel" system: Strongly typed "external" (Nyzo) messages and untyped
internal messages. The above modules subscribe to the messages they need and act on them independently. All messages are handled 
through the message router (nyzo/router), which is able to multicast messages. 

Internally to the individual modules, messages often serve an additional purpose: they serialize data access and thus 
keep us from having to use mutexes in many cases. So it can well be that some module sends itself an internal, unrouted message to retrieve
some type of information via its own sequential messaging loop.

##### Other Components and Special Cases
One of the most important recent changes to the node structure is that we turned the actual block file storage and handling into
directly called functions instead of functions effected via message passing. This essentially sequentializes the operation of the
entire node around the progress of the blockchain up to the frozen edge. Handling the ultimate truth of the chain via less
tightly bound mechanisms lead to all kinds of issues, especially when quickly processing large amounts of historical blocks. 

Several components of the system are not part of the above blockchain context model, they usually serve simple, clearly delineated
purposes like message routing, reading and writing a key-value store etc. 

## Philosophical Part

### Why Go?
Go is well-established in the blockchain space (Ethereum, for example, is mostly written in Go). It is a modern 
language with enough history and support to appear like a sustainable choice. Its concurrency patterns are ideal 
for our needs. Furthermore, Go is one of the few languages that has the potential to enable performance gains 
relative to the Java implementation.

### Project Goal
The goal of this project is to implement a fully compatible Nyzo verifier in Golang. The ultimate test of whether 
we achieved this goal will be that a running Java verifier can be stopped, the Go verifier can be started, and 
it should seamlessly take over and continue to serve the network.

On the way to this goal, it appears useful to release an MVP, which will be a blockchain observer node, providing 
for example transaction history data that is currently very hard to access within the Nyzo system.

### Implementation Philosophy
At first sight, it appears like a good idea to keep the original Java program and the Go program structures close. 
However, Go and Java are sufficiently different to make this a non-starter, so we aim for feature completeness 
and compatibility, rather than for similarity in implementation.

## Working Style & Collaboration

### Background and MVP Phase
I am a sufficiently experienced programmer, but I did learn Go for this project, and I shaped its architecture while
trying to grasp the full complexity of the Java node's architecture. For that reason, some early code fragments might
be a little less idiomatic than the later ones. During the MVP phase, focus should be on getting the structure right,
instead of on not breaking anything at any price. 

### Commenting and Coding Style
Code should be reasonably commented, every larger code entity should have a top-level explanation of what it does, and
special "gotchas" or potentially hard to dissect code segments should be commented accordingly. Within the limits of the
time I had, I tried to learn and write idiomatic Go according to the official language spec. With a few notable
exceptions, I tried to avoid idiosyncratic, but hard to read Go statements like the use of initializers in "if" statements:

	if err := configuration.EnsureSetup(); err != nil {
		logging.ErrorLog.Fatal(err.Error())
	}
	
### Testing
Aiming for a reasonably high test coverage (not there yet). Not all aspects can be tested "hors sol" in such a complex
system, so letting the node catch up to the frozen edge from genesis is one of the things I currently do quite regularly.
Thanks to the straitjacket of the blockchain's hashing and signature verification mechanisms, blockchain versions up to
V2 should now be reasonably covered up to the frozen edge, any errors occurring should be pretty deterministic. 

Apart from writing unit tests, the project will later have to operate an (ideally mixed) testnet, a reasonable first test will
be to see whether the Go node can act as a working Sentinel.

Tests are allowed to look nasty as long as they test what they intend to test.

### Cooperation
If you are interested in contributing to Go-Nyzo, please contact me (@cryptic_monk on Twitter, Monk#4056 on Discord). 
For any changes, please create a feature branch off the development branch and submit a pull request from there. 
