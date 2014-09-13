ahimsad
=======


A Bitcoin daemon that builds and maintains a sqlite database built from Bitcoin's blockchain.
ahimsad processes the blockchain looking for messages stored in our format. 
It also connects to a live Bitcoin node and listens for new messages forwarded over the network.
We think that storing small messages in blockchains is a real usecase for a distributed timestamp authority (e.g. Bitcoin). 


Installation
==========
If you are running a 64-bit linux distribution then the install will work otherwise you are out of luck. 
There is a script to install all of these dependencies in one go that
lives [here](http://github.com/NSkelsey/protocol/blob/master/deploy/install_everything.sh).
Bitcoin takes a while to download the blockchain so just remember to exercise your patience.
It is not in our scope to explain how to install Bitcoin, so we are just going to describe
the necessary tweaks that are needed with a default install.

Automatic! 
----------

You can run a script to install everything needed for  ahimsad. 
This script assumes a lot about your system.
It actually assumes that you are using _my_ system.
I run Ubuntu 14.04 servers, so this will probably work on that.

```bash
$ wget https://raw.githubusercontent.com/NSkelsey/protocol/master/deploy/install_everything.sh
$ source install_everything.sh; ahimsad_deps
```

Manual!
-------

####Install and configure bitcoin. 

This is the longest step by far.
A typical bitcoin.conf file for a system that runs ahimsad looks like:

```
rpcuser=[your-user]
rpcpassword=[your-password]
testnet=1
server=1
```

####Install and configure go. 

Set $GOPATH, $GOROOT and add $GOPATH/bin to your $PATH.
Google has great instructions regarding how to do this.

####Download and build ahimsad
```bash
$ apt get install mercurial git
$ go get github.com/NSkelsey/ahimsad/...
```
Run it once to see if it works. If everything went well then it will complain about 
missing settings.
```bash
$ ahimsad
You need to correctly set rpcuser and rpcpassword for ahimsad to work properly.
Additionally check to see if you are using the TestNet or MainNet.
```

####Configure ahimsad
See [sample.conf](https://github.com/NSkelsey/ahimsad/blob/master/sample.conf) for sample 
configurations of ahimsad. 

When bitcoind has caught up to the longest valid chain you are ready to run ahimsasd!

```bash
$ ahimsad
```


####Notes

All that is required in ~/.ahimsa/ahimsa.conf to run ahimsad on testnet is:
```
rpcuser=[same-as-above]
rpcpassword=[same-as-above]
```

- bitcoind must be running for ahimsad to function.
- The initial construction of the pubrecord.db from blockfiles should take around 2 hours on Mainnet.
