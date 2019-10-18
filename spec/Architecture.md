# Architecture

## The Handler Interface

For the work below, we are just looking at the "Handler" interface. In go, this is `type Handler func(ctx Context, msg Msg) Result`. In addition to the message to process, it gets a Context that allows it to view and mutate state:

```go
func (c Context) Context() context.Context    { return c.ctx }
func (c Context) MultiStore() MultiStore      { return c.ms }
func (c Context) BlockHeight() int64          { return c.header.Height }
func (c Context) BlockTime() time.Time        { return c.header.Time }
func (c Context) ChainID() string             { return c.chainID }
func (c Context) TxBytes() []byte             { return c.txBytes }
func (c Context) Logger() log.Logger          { return c.logger }
func (c Context) VoteInfos() []abci.VoteInfo  { return c.voteInfo }
func (c Context) GasMeter() GasMeter          { return c.gasMeter }
func (c Context) BlockGasMeter() GasMeter     { return c.blockGasMeter }
func (c Context) IsCheckTx() bool             { return c.checkTx }
func (c Context) MinGasPrices() DecCoins      { return c.minGasPrice }
func (c Context) EventManager() *EventManager { return c.eventManager }
```

It is clearly not desirable to expose all this to arbitrary, unaudited code, but we can grab a subset of this to expose to the wasm contract.

Readonly:

* Block Context
    * BlockHeight
    * BlockTime
    * ChainID
* Message Data
    * Signer (who authorized this message)
    * Tokens Sent with message
    * User-defined data
* Contract State
    * Contract Address
    * Contract Account (just balance or more info?)

Read/Write:

* A sandboxed sub-store
* Events

### Security Concerns

We clearly cannot just pass in a Controller to another module into an unknown wasm contract. But we do need some way to allow a wasm contract to integrate with other modules to make it interesting. Above we allow it access to its internal state and ability to read its own balance.

In terms of security, we can view the wasm contract as a "client" on-chain with its  own account and address. It can theoretically read any on-chain data, and send any message  "signed" by its own address, without opening up security holes. Provided these messages are processed just like external messages, and that gas limits are enforced in CPU time and  those queries.

### Calling Other Modules

Ethereum provides a nice dispatch model, where a contract can make arbitrary calls to the public API of any other contract. However, we have seen many issues and bugs, especially related to re-entrancy attacks. To simplify this, we propose that the contract cannot directly call any other contract, but instead returns a list of messages, which will be dispatched and validated *after contract execution* but in *the same transaction*. This means that if they fail, the contract will also roll back, but we don't allow any cycles or re-entrancy possibilities.

We could conceive of this as something like: `ProcessMessage(info ReadOnlyInfo, db SubStore) []Msg`. Note that we also want to allow it to return a `Result` and `Events`, so this may end up with a much larger pseudo-function signature, like: `ProcessMessage(info ReadOnlyInfo, db SubStore) (*Result, []Event, []Msg, error)`

This allows the contract to easily move the tokens it controls (via `SendMsg`) or even vote, stake tokens,  or take any other action its account has authority to do. The potential actions increase with the delegation work being done as part of Key Managament.

### Querying Other Modules

While it is great to change state in other modules, the design until this point leave the contract blind. Sure, it can emit a message in order to stake some tokens, but it cannot check its current stake, or the number of tokens available to withdraw. To do so, we need to expose some interface to query other modules.

Going along with the client analogy above , we definitely **do not** want to allow *write* access to the other substores. Instead we can allow something like the high level  `abci_query` interface, where the smart contract would send a path and data (key), and receive the requested object - likely serialized as json rather than amino for ease
of parsing in the smart contract.

Whether we wrap the existing `Query` interface or provide a different interface just for WASM contract is an open question, which we touch in the next section.

## Upgradeability

This can be a **major problem** or even **blocker** for enabling Web Assembly contracts, unless we make some very conscious design decisions, both in the WASM interfaces, as well as the SDK as a whole.

If we allow the contract to query arbitrary data in other modules, this contract is dependent on those not changing. If the chain upgrades (gracefully or hardfork) and the queries return data in a different format, or change the path they respond to, then the contract will break. The same is true with the format of the messages we return. If cosmos-sdk modifies the format of the staking message, after an upgrade the module will continue to emit  staking messages in the old format, which will fail - leave the contract broken and fund stuck.

One proposed solution was to allow us to "upgrade contracts" as well, but I find this highly problematic. A core pillar of most smart contract designs is immutability, which is what allows us to trust them. If the author could change them *after* I send it my funds, then there can be no trustless execution. Maybe we then decide that governance can update the contracts, or only change them in the context of a hard-fork. This provides safe-guards, but the issue arises that the contract author and those updating the application code, and the validators all are different entities. Do we now need to contact every contract author and involve them in preparing every upgrade? Or will the validators just re-wrire contracts as they see fit? Seems extremely risky in any case.

One alternative here is to either freeze every API that a Handler touches in cosmos-sdk, as well as every data structure exposed over `abci_query`. But this would have the effect of a huge stagnation of the codebase and very determental to innovation.

Another alternative, and what we propose here, is to limit the interfaces exposed to the wasm contracts to a minimalistic subset of all possible functionality. And provide a fixed format with strong immutability guarantees. This will likely require some wrapper between the structs used in the wasm interface, and those used elsewhere in the sdk. As we noticed, even the transaction type changed in the upgrade to Gaia 2.0.

We could, for example, expose a custom SendMsg, `{type: 'send', to, from, amount}` and then in the Golang wrapper code (which can be updated easily during a hard-fork), we translate this *well-defined*, *immutable*, and *forward-comaptible* message definition into the actual structure used by the cosmos-sdk, before dispatching this to other modules. We would like-wise have to provide clear definitions for a minimal set of queries we want to expose, and then make sure to translate fields from the current struct into this static definition.

This means we would have to manually enable each Message or Query we would want to expose to all Web Assembly contract, and provide strong guarantees to each of them *forever*. We could easily add new message types, or queries, such that new contracts deployed after version X could  make use of them, but all the types that were exposed to the first contract deployed on the system must remain valid for the lifetime of the chain (including any hardforks, dump-state-and-reset, etc.).

**WARNING**

Even with a buffer class, this will have a noticeable strong impact on a number of development practices in the core cosmos-sdk team, especially related to version and migration, and we need to have a clear and open discussion on possible approaches here.

Relevant link (recommended by Aaron): https://github.com/matthiasn/talk-transcripts/blob/master/Hickey_Rich/Spec_ulation.md 
