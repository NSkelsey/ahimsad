ahimsad
=======

A Bitcoin daemon that implements the core interfaces of the ahimsa protocol.
ahimsa's sole purpose is to encode and decode messages in blockchains. The messages
format is defined in a series of protocol buffers that are encoding in the TxOuts
of transactions.

We think storing tweet sized messages in blockchains is a potential usecase for a
distributed timestamp authority (like the Bitcoin network). Occasionally, you may want to store your online statements permanently
and publicly. If you are willing to pay miners for that storage ahimsad will let you 
do it.


Installing
==========
If you are running a 64-bit linux distribution then follow these intstuctions.
Alternatively there is a script to install all of these dependencies in one go that
lives [here](http://github.com/NSkelsey/protocol/blob/master/deploy/install_everything.sh).

You can run it to install ahimsad with:

```
source install_everything.sh; ahimsad_deps
```

Install and configure bitcoin.
A typical bitcoin.conf file looks like:

```
rpcuser=[your-user]
rpcpassword=[your-password]
testnet=1
server=1
txindex=1
```

Install and configure go. 

Set GOPATH, GOROOT and add $GOPATH/bin to your $PATH

Download and build ahimsad:
```bash
apt get install mercurial git
go get github.com/NSkelsey/ahimsad/...
```
Run it once to see if it works. If everything went well then it will complain about 
missing settings.
```bash
ahimsad
```


If that worked, then its time to configure it. See sample.conf for sample 
configurations for ahimsad. This is the point where you need to decide whether 
you are going to run the ahimsad on testnet or mainnet.

All that is required to run on testnet is:
```
rpcuser=[same-as-above]
rpcpassword=[same-as-above]
```

Run bitcoin:
```bash
bitciond
```

Let the blockchain download or kickstart the process by downloading a checkpoint.
When bitcoind reports that it is caught up, you are ready to run ahimsasd!

```bash
ahimsad
```

todo
=====

Remove txindex requirement.
    - author pubkey lives in txin of given tx.

