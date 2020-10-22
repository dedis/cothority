import * as Long from "long";
import { BehaviorSubject } from "rxjs";
import { map } from "rxjs/operators";

import { Argument, ByzCoinRPC, Instance as BCInstance, InstanceID } from "../../../byzcoin";
import { Coin } from "../../../byzcoin/contracts";
import Log from "../../../log";

import { TransactionBuilder } from "..";
import { ObservableToBS } from "..";
import { CoinContract } from "./";

/**
 * CoinStruct merges a Coin structure with an instance.
 */
export class CoinStruct extends Coin {
    constructor(readonly inst: BCInstance) {
        super(Coin.decode(inst.data));
    }
}

/**
 * CoinBS represents a coin with the new interface. Instead of relying on a synchronous interface,
 * this implementation allows for a more RxJS-style interface.
 */
export class CoinInst extends BehaviorSubject<CoinStruct> {

    /**
     * Retrieves a coinInstance from ByzCoin and returns a BehaviorSubject that updates whenever the
     * coin changes.
     *
     * @param bc of an initialized ByzCoinRPC instance
     * @param coinID of an existing coin instance
     * @return a BehaviorSubject pointing to a coinInstance that updates automatically
     */
    static async retrieve(bc: ByzCoinRPC, coinID: InstanceID):
        Promise<CoinInst> {
        Log.lvl3("getting coinBS");
        const coinObs = (await bc.instanceObservable(coinID)).pipe(
            map((proof) => new CoinStruct(BCInstance.fromProof(coinID, proof))),
        );
        return new CoinInst(await ObservableToBS(coinObs));
    }

    readonly id: InstanceID;

    constructor(coin: BehaviorSubject<CoinStruct>) {
        super(coin.getValue());
        coin.subscribe(this);
        this.id = coin.getValue().inst.id;
    }

    /**
     * Creates an instruction to transfer coins to another account.
     *
     * @param tx used to collect one or more instructions that will be bundled together and sent as one transaction
     * to byzcoin.
     * @param dest the destination account to store the coins in. The destination must exist!
     * @param amount how many coins to transfer.
     */
    transfer(tx: TransactionBuilder, dest: InstanceID, amount: Long) {
        CoinContract.transfer(tx, this.getValue().inst.id, dest, amount);
    }

    /**
     * Mints coins on ByzCoin. For this to work, the DARC governing this coin instance needs to have a
     * 'invoke.Coin.mint' rule and the instruction will need to be signed by the appropriate identity.
     *
     * @param tx used to collect one or more instructions
     * @param amount positive, non-zero value to mint on this coin
     * @return the coin as it will be created if the transaction is accepted - warning: other instructions in this
     * transaction might change the value of the coin.
     */
    mint(tx: TransactionBuilder, amount: Long): Coin {
        const ci = this.getValue();
        CoinContract.mint(tx, ci.inst.id, amount);
        return new Coin({name: ci.name, value: ci.value.add(amount)});
    }

    /**
     * Fetches coins from a coinInstance and puts it on the ByzCoin 'stack' for use by the next instruction.
     * Unused coins are discarded by all nodes and thus lost.
     *
     * @param tx used to collect one or more instructions that will be bundled together and sent as one transaction
     * to byzcoin.
     * @param amount how many coins to put on the stack
     */
    fetch(tx: TransactionBuilder, amount: Long) {
        tx.invoke(this.getValue().inst.id, CoinContract.contractID, CoinContract.commandFetch,
            [new Argument({name: CoinContract.argumentCoins, value: Buffer.from(amount.toBytesLE())})]);
    }

    /**
     * Stores coins from the ByzCoin 'stack' in the given coin-account. Only the coins of the stack with the same
     * name are added to the destination account.
     *
     * @param tx used to collect one or more instructions that will be bundled together and sent as one transaction
     * to byzcoin.
     */
    store(tx: TransactionBuilder) {
        tx.invoke(this.getValue().inst.id, CoinContract.contractID, CoinContract.commandStore, []);
    }
}
