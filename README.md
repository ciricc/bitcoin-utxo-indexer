# Bitcoin UTXO indexer

Yea, this is yet another bitcoin UTXO indexer. 
Just an example, not a production-ready project.

## The main idea and real "Why"

This UTXO indexer was supposed to be universal UTXO indexer for any bitcoin compatible blockchain. 

This project was developed for high read speed. So, i chose the [https://www.dragonflydb.io/](Dragonfly) as the main Redis compatible storage, it is x25 faster then Redis and x5 faster then KeyDB. Also, it is storing the data more eficciently. So, i have reduced the RAM usage by -10% compare to Redis/KeyDB. using Dragonfly, you can start up this UTXO indexer in 1 hour of your life instead of 1 day using Redis.

This project was developped with priority on durability. It has some problems with the consistency like: if the blockchain node got new block, but the UTXO service not synced it yet, then it will return old UTXO. 

New UTXO adds from the checkpoint files (save points) which stores durable information: what it was before, what is needed to be updated.

## Architecture of the project

The project consists of the following components: 

1. **API level**
- gRPC transport as the main transport of the UTXO service

2. **Service level** 
- UTXO service - the service of the UTXO's

3. **Storage**
- Redis compatible key-value storage for storing all UTXO's. I have tested syncing from the chainstate snapshot with the Dragonfly and it was x25 faster then Redis and x5 faster then KeyDB. You can start up this project in 1 hour and be ready for storing all UTXO set in your RAM. This storages stores some information:
```
- UTXO set (indexed by address, indexed by transaction hash)
- Migration information (current version, list of the versions)
- Blockchain information (block height, best hash etc)
```

4. **Daemons**
- Chainstate migration tool for upload all already synced UTXO from the blockchain node faster then by http downloading blocks.
- Service which syncing with the Bitcoin node by the REST interface (blokchain scanner, blocks iterator, universal bitcoin rest client)

5. **Utilities and abstractions**
- Transaction manager (redis and leveldb)
- Sets abstraction (for storing the sets, used for storing the UTXO set)

## System requirements

- 48 GB RAM (transaction indexes + adddress indexes)
- Bitcoin core fully synced node with enabled flag `-rest=1`


## Warning

This project has the code not used anymore.
If you interested in this project (but i dont think so), write me in any time: https://t.me/boysdontcrie

## Maybe later

- Fix high RAM usage (transaction index should be stored on the disk insted of RAM)
- Fix consistency problems (maybe the service should return error if this is not synced yet)
