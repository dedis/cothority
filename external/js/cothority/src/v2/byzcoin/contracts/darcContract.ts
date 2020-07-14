import { randomBytes } from "crypto-browserify";

import { Argument, InstanceID } from "../../../byzcoin";
import { Darc } from "../../../darc";

import { TransactionBuilder } from "../transactionBuilder";

import { CoinContract } from "./";
import { IDarcAttr } from "./darcInsts";

/**
 * DarcContract represents a darc taken from an instance. It has all necessary constants to interact with a darc
 * contract on byzcoin.
 */
export const contractID = "darc";
export const commandEvolve = "evolve";
export const commandEvolveUnrestricted = "evolve_unrestricted";
export const argumentDarc = "darc";
export const ruleSign = Darc.ruleSign;
export const ruleEvolve = "invoke:" + contractID + "." + commandEvolve;
export const ruleEvolveUnrestricted = "invoke:" + contractID + "." +
    commandEvolveUnrestricted;

/**
 * Creates an instruction in the transaction with either an update of the description and/or an update
 * of the rules.
 *
 * @param tx where the instruction will be appended to
 * @param oldDarc that needs to be evolved
 * @param updates contains a description update and/or rules to be merged
 * @param unrestricted if true, will create an unrestricted evolve that allows to create new rules
 * @return the new DARC as it will appear on ByzCoin if the transaction is accepted
 */
export function evolve(tx: TransactionBuilder, oldDarc: Darc, updates: IDarcAttr, unrestricted = false): Darc {
    const newArgs = {...oldDarc.evolve(), ...updates};
    const newDarc = new Darc(newArgs);
    const cmd = unrestricted ? commandEvolveUnrestricted : commandEvolve;
    const args = [new Argument({
        name: argumentDarc,
        value: Buffer.from(Darc.encode(newDarc).finish()),
    })];
    tx.invoke(newDarc.getBaseID(), contractID, cmd, args);
    return newDarc;
}

/**
 * Creates the instruction necessary to spawn a new coin using this darc.
 *
 * @param tx where the instruction will be appended to
 * @param did baseID of the darc that can spawn coins
 * @param name of the coin
 * @param preHash used to calculate the ID of the coin, if given
 */
export function spawnCoin(tx: TransactionBuilder, did: InstanceID, name: Buffer, preHash?: Buffer): InstanceID {
    if (preHash === undefined) {
        preHash = randomBytes(32);
    }
    const args = [new Argument({name: CoinContract.argumentType, value: name}),
        new Argument({name: CoinContract.argumentCoinID, value: preHash})];
    tx.spawn(did, CoinContract.contractID, args);
    return CoinContract.coinIID(preHash);
}

/**
 * Spawns a new darc. The darc given by did must have a `spawn:darc` rule.
 *
 * @param tx where the instruction will be appended to
 * @param did baseID of the darc that can spawn other darcs
 * @param newDarc to be spawned
 */
export function spawnDarc(tx: TransactionBuilder, did: InstanceID, newDarc: Darc) {
    tx.spawn(did, contractID,
        [new Argument({name: argumentDarc, value: newDarc.toBytes()})]);
}
