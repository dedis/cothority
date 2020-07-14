import { BehaviorSubject } from "rxjs";
import { flatMap, map } from "rxjs/operators";

import { Argument, ByzCoinRPC, Instance, InstanceID } from "../../../byzcoin";
import { Darc, IIdentity, Rule, Rules } from "../../../darc";
import IdentityDarc from "../../../darc/identity-darc";
import Log from "../../../log";

import { TransactionBuilder } from "..";
import { ObservableToBS } from "..";

import { DarcContract } from "./";

/**
 * Used in DarcBS.evolve for chosing which parts of the DARC to evolve.
 */
export interface IDarcAttr {
    description?: Buffer;
    rules?: Rules;
}

/**
 * Update rules given an action and an identity. If it's an InstanceID, it will
 * be interpreted as darc:instanceID.
 */
export type IRule = [string, IIdentity | InstanceID];

/**
 * DarcStruct holds a darc together with the corresponding instance. This instance can be used to
 * get the version.
 */
export class DarcStruct extends Darc {
    constructor(readonly inst: Instance) {
        super(Darc.decode(inst.data));
    }
}

/**
 * Holds a list of DARCs that will be updated individually, and whenever the list changes.
 */
export class DarcInsts extends BehaviorSubject<DarcInst[]> {

    /**
     * Retrieves an eventually changing list of darcs from ByzCoin.
     *
     * @param bc of an initialized ByzCoinRPC instance
     * @param idsBS
     */
    static async retrieve(bc: ByzCoinRPC, idsBS: BehaviorSubject<InstanceID[]> | InstanceID[]): Promise<DarcInsts> {
        Log.lvl3("getting darcsBS");
        if (idsBS instanceof Array) {
            idsBS = new BehaviorSubject(idsBS);
        }
        const darcs = await ObservableToBS(idsBS.pipe(
            flatMap((ais) => Promise.all(ais
                .map((iid) => DarcInst.retrieve(bc, iid)))),
            map((dbs) => dbs.filter((db) => db !== undefined)),
        ));
        return new DarcInsts(darcs);
    }

    constructor(sbs: BehaviorSubject<DarcInst[]>) {
        super(sbs.getValue());
        sbs.subscribe(this);
    }
}

/**
 * A DarcBS class represents a darc on byzcoin. It has methods to modify the darc by
 * adding and removing rules, as well as to change the description.
 */
export class DarcInst extends BehaviorSubject<DarcStruct> {

    /**
     * Retrieves a DarcBS from ByzCoin given an InstanceID.
     *
     * @param bc of an initialized ByzCoinRPC instance
     * @param darcID a fixed InstanceID representing the baseID of the darc to retrieve
     * @return a DarcBS or undefined if something went wrong (no Darc at that ID)
     */
    static async retrieve(bc: ByzCoinRPC, darcID: InstanceID):
        Promise<DarcInst> {
        Log.lvl3("getting darcBS");
        const instObs = (await bc.instanceObservable(darcID)).pipe(
            map((proof) => (proof && proof.value && proof.value.length > 0) ?
                new DarcStruct(Instance.fromProof(darcID, proof)) : undefined),
        );
        const bsDarc = await ObservableToBS(instObs);
        if (bsDarc.getValue() === undefined) {
            throw new Error("this darc doesn't exist");
        }
        return new DarcInst(bsDarc);
    }

    constructor(darc: BehaviorSubject<DarcStruct>) {
        super(darc.getValue());
        darc.subscribe(this);
    }

    /**
     * Creates an instruction in the transaction with either an update of the description and/or an update
     * of the rules.
     *
     * @param tx where the instruction will be appended to
     * @param updates contains a description update and/or rules to be merged
     * @param unrestricted if true, will create an unrestricted evolve that allows to create new rules
     * @return the new DARC as it will appear on ByzCoin if the transaction is accepted
     */
    evolve(tx: TransactionBuilder, updates: IDarcAttr, unrestricted = false): Darc {
        return DarcContract.evolve(tx, this.getValue(), updates, unrestricted);
    }

    /**
     * Sets the description of the DARC.
     *
     * @param tx where the instruction will be appended to
     * @param description of the new DARC
     * @return the new DARC as it will appear on ByzCoin if the transaction is accepted
     */
    setDescription(tx: TransactionBuilder, description: Buffer): Darc {
        return this.evolve(tx, {description});
    }

    /**
     * Creates a new darc by overwriting existing rules.
     * A darc with a rule for `invoke:darc.evolve_unrestricted` can also accept new rules
     *
     * @param tx where the instruction will be appended to
     * @param newRules is a map of action to expression
     * @return the new DARC as it will appear on ByzCoin if the transaction is accepted
     */
    setRules(tx: TransactionBuilder, ...newRules: IRule[]): Darc {
        const rules = this.getValue().rules.clone();
        newRules.forEach(([action, expression]) => rules.setRule(action, toIId(expression)));
        return this.evolve(tx, {rules});
    }

    /**
     * Adds a sign and evolve element to the DARC with an OR expression
     *
     * @param tx where the instruction will be appended to
     * @param addRules is a map of actions to identities that will be ORed with the existing expression.
     * @return the new DARC as it will appear on ByzCoin if the transaction is accepted
     */
    addToRules(tx: TransactionBuilder, ...addRules: IRule[]): Darc {
        const rules = this.getValue().rules.clone();
        addRules.forEach(([action, expression]) => rules.appendToRule(action, toIId(expression), Rule.OR));
        return this.evolve(tx, {rules});
    }

    /**
     * Removes an identity in the sign and/or evolve expression. The expressions need to be pure
     * OR expressions, else this will fail.
     *
     * @param tx where the instruction will be appended to
     * @param rmRules is a map of actions to identities that will be removed from existing rules.
     * @return the new DARC as it will appear on ByzCoin if the transaction is accepted
     */
    rmFromRules(tx: TransactionBuilder, ...rmRules: IRule[]): Darc {
        const rules = this.getValue().rules.clone();
        for (const [action, expression] of rmRules) {
            try {
                rules.getRule(action).remove(toIId(expression).toString());
            } catch (e) {
                Log.warn("while removing identity from ", action, ":", e);
            }
        }
        return this.evolve(tx, {rules});
    }

    /**
     * Creates the instruction necessary to spawn a new coin using this darc.
     *
     * @param tx where the instruction will be appended to
     * @param name of the coin
     * @param preHash used to calculate the ID of the coin, if given
     */
    spawnCoin(tx: TransactionBuilder, name: Buffer, preHash?: Buffer): InstanceID {
        return DarcContract.spawnCoin(tx, this.getValue().getBaseID(), name, preHash);
    }

    /**
     * Spawns a new darc. The current darc must have a `spawn:darc` rule.
     *
     * @param tx where the instruction will be appended to
     * @param newDarc to be spawned
     */
    spawnDarc(tx: TransactionBuilder, newDarc: Darc) {
        tx.spawn(this.getValue().getBaseID(), DarcContract.contractID,
            [new Argument({name: DarcContract.argumentDarc, value: newDarc.toBytes()})]);
    }
}

function toIId(id: IIdentity | InstanceID): IIdentity {
    if (Buffer.isBuffer(id)) {
        return new IdentityDarc({id});
    }
    return id;
}
