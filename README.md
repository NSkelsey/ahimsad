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
