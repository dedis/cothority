import Long from "long";
import { Message, Properties } from "protobufjs/light";
import Signer from "../../darc/signer";
import { EMPTY_BUFFER, registerMessage } from "../../protobuf";
import ByzCoinRPC from "../byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../client-transaction";
import { InstanceID } from "../instance";
import { SPAWNER_COIN } from "./spawner-instance";

export default class CoinInstance {
    static readonly contractID = "coin";

    /**
     * Create a coin instance from a darc id
     *
     * @param bc        The RPC to use
     * @param darcID    The darc instance ID
     * @param signers   The list of signers for the transaction
     * @param type      The coin instance type
     * @returns a promise that resolves with the new instance
     */
    static async create(
        bc: ByzCoinRPC,
        darcID: InstanceID,
        signers: Signer[],
        type: Buffer = SPAWNER_COIN,
    ): Promise<CoinInstance> {
        const inst = Instruction.createSpawn(
            darcID,
            CoinInstance.contractID,
            [new Argument({ name: "type", value: type })],
        );
        await inst.updateCounters(bc, signers);

        const ctx = new ClientTransaction({ instructions: [inst] });
        ctx.signWith(signers);

        await bc.sendTransactionAndWait(ctx, 10);

        const coinIID = inst.deriveId();
        const p = await bc.getProof(coinIID);
        if (!p.exists(coinIID)) {
            throw new Error("fail to get a matching proof");
        }

        return new CoinInstance(bc, coinIID, Coin.decode(p.value));
    }

    /**
     * Initializes using an existing coinInstance from ByzCoin
     * @param bc    The RPC to use
     * @param iid   The instance ID
     * @returns a promise that resolves with the coin instance
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<CoinInstance> {
        const proof = await bc.getProof(iid);
        if (!proof.exists(iid)) {
            throw new Error(`key not in proof: ${iid.toString("hex")}`);
        }

        return new CoinInstance(bc, iid, Coin.decode(proof.value));
    }

    private rpc: ByzCoinRPC;
    private iid: Buffer;
    private coin: Coin;

    constructor(bc: ByzCoinRPC, iid: Buffer, coin: Coin) {
        this.rpc = bc;
        this.iid = iid;
        this.coin = coin;
    }

    /**
     * Getter for the instance id
     * @returns the id
     */
    get id() {
        return this.iid;
    }

    /**
     * Getter for the coin name
     * @returns the name
     */
    get name(): Buffer {
        return this.coin.name;
    }

    /**
     * Getter for the coin value
     * @returns the value
     */
    get value(): Long {
        return this.coin.value;
    }

    /**
     * Get the coin object
     * @returns the coin object
     */
    getCoin(): Coin {
        return this.coin;
    }

    /**
     * Transfer a certain amount of coin to another account.
     *
     * @param coins     the amount
     * @param to        the destination account (must be a coin contract instance id)
     * @param signers   the signers (of the giver account)
     */
    async transfer(coins: Long, to: Buffer, signers: Signer[]): Promise<void> {
        const args = [
            new Argument({ name: "coins", value: Buffer.from(coins.toBytesLE()) }),
            new Argument({ name: "destination", value: to }),
        ];

        const inst = Instruction.createInvoke(this.iid, CoinInstance.contractID, "transfer", args);
        await inst.updateCounters(this.rpc, signers);

        const ctx = new ClientTransaction({ instructions: [inst] });
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx, 10);
    }

    /**
     * Mine a given amount of coins
     *
     * @param signers   The list of signers for the transaction
     * @param amount    The amount to add to the coin instance
     * @param wait      Number of blocks to wait for inclusion
     */
    async mint(signers: Signer[], amount: Long, wait?: number): Promise<void> {
        const inst = Instruction.createInvoke(
            this.iid,
            CoinInstance.contractID,
            "mint",
            [new Argument({ name: "coins", value: Buffer.from(amount.toBytesLE()) })],
        );
        await inst.updateCounters(this.rpc, signers);

        const ctx = new ClientTransaction({ instructions: [inst] });
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx, wait);
    }

    /**
     * Update the data of this instance
     *
     * @returns the updated instance
     */
    async update(): Promise<CoinInstance> {
        const p = await this.rpc.getProof(this.iid);
        if (!p.exists(this.iid)) {
            throw new Error("fail to get a matching proof");
        }

        this.coin = Coin.decode(p.value);
        return this;
    }
}

export class Coin extends Message<Coin> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.Coin", Coin);
    }

    name: Buffer;
    value: Long;

    constructor(props?: Properties<Coin>) {
        super(props);

        this.name = Buffer.from(this.name || EMPTY_BUFFER);
    }

    toBytes(): Buffer {
        return Buffer.from(Coin.encode(this).finish());
    }
}

Coin.register();
