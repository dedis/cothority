import { createHash } from "crypto-browserify";
import * as Long from "long";

import { TransactionBuilder } from "..";
import { Argument, InstanceID } from "../../../byzcoin";

export const contractID = "coin";
export const commandMint = "mint";
export const commandFetch = "fetch";
export const commandTransfer = "transfer";
export const commandStore = "store";
export const argumentCoinID = "coinID";
export const argumentDarcID = "darcID";
export const argumentType = "type";
export const argumentCoins = "coins";
export const argumentDestination = "destination";
export const ruleSpawn = "spawn:" + contractID;
export const ruleMint = rule(commandMint);
export const ruleFetch = rule(commandFetch);
export const ruleTransfer = rule(commandTransfer);
export const ruleStore = rule(commandStore);

/**
 * Generate the coin instance ID for a given darc ID
 *
 * @param buf Any buffer that is known to the caller
 * @returns the id as a buffer
 */
export function coinIID(buf: Buffer): InstanceID {
    const h = createHash("sha256");
    h.update(Buffer.from(contractID));
    h.update(buf);
    return h.digest();
}

/**
 * Mints coins on ByzCoin. For this to work, the DARC governing this coin instance needs to have a
 * 'invoke.Coin.mint' rule and the instruction will need to be signed by the appropriate identity.
 *
 * @param tx used to collect one or more instructions
 * @param coinID to mint
 * @param amount positive, non-zero value to mint on this coin
 */
export function mint(tx: TransactionBuilder, coinID: InstanceID, amount: Long) {
    if (amount.lessThanOrEqual(0)) {
        throw new Error("cannot mint 0 or negative values");
    }
    tx.invoke(coinID,
        contractID,
        commandMint,
        [new Argument({name: argumentCoins, value: Buffer.from(amount.toBytesLE())})]);
}

/**
 * Creates an instruction to transfer coins to another account.
 *
 * @param tx used to collect one or more instructions that will be bundled together and sent as one transaction
 * to byzcoin.
 * @param src the source account to fetch coins from.
 * @param dest the destination account to store the coins in. The destination must exist!
 * @param amount how many coins to transfer.
 */
export function transfer(tx: TransactionBuilder, src: InstanceID, dest: InstanceID, amount: Long) {
    tx.invoke(src, contractID, commandTransfer,
        [new Argument({name: argumentDestination, value: dest}),
            new Argument({name: argumentCoins, value: Buffer.from(amount.toBytesLE())})]);
}

/**
 * Fetches coins from a coinInstance and puts it on the ByzCoin 'stack' for use by the next instruction.
 * Unused coins are discarded by all nodes and thus lost.
 *
 * @param tx used to collect one or more instructions that will be bundled together and sent as one transaction
 * to byzcoin.
 * @param src the source account to fetch coins from.
 * @param amount how many coins to put on the stack
 */
export function fetch(tx: TransactionBuilder, src: InstanceID, amount: Long) {
    tx.invoke(src, contractID, commandFetch,
        [new Argument({name: argumentCoins, value: Buffer.from(amount.toBytesLE())})]);
}

/**
 * Stores coins from the ByzCoin 'stack' in the given coin-account. Only the coins of the stack with the same
 * name are added to the destination account.
 *
 * @param tx used to collect one or more instructions that will be bundled together and sent as one transaction
 * to byzcoin.
 * @param dst where the coins to store
 */
export function store(tx: TransactionBuilder, dst: InstanceID) {
    tx.invoke(dst, contractID, commandStore, []);
}

function rule(command: string): string {
    return `invoke:${contractID}.${command}`;
}
