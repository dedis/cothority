import Long from "long";
import { Message } from "protobufjs";
import ByzCoinRPC from "../byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../client-transaction";
import Signer from "../../darc/signer";
import { SpawnerCoin } from "./spawner-instance";
import Proof from "../proof";
import { registerMessage } from "../../protobuf";

export default class CoinInstance {
    static readonly contractID = "coin";

    private rpc: ByzCoinRPC;
    private iid: Buffer;
    private coin: Coin;

    constructor(bc: ByzCoinRPC, iid: Buffer, coin: Coin) {
        this.rpc = bc;
        this.iid = iid;
        this.coin = coin;
    }

    get id() {
        return this.iid;
    }

    get name(): Buffer {
        return this.coin.name;
    }

    get value(): Long {
        return this.coin.value;
    }

    /**
     * Transfer a certain amount of coin to another account.
     *
     * @param  coins the amount
     * @param to the destination account (must be a coin contract instance id)
     * @param signers the signers (of the giver account)
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

    async mint(signers: Signer[], amount: Long, wait?: number): Promise<void> {
        const inst = Instruction.createInvoke(
            this.iid,
            CoinInstance.contractID,
            "mint",
            [new Argument({ name: "coins", value: Buffer.from(amount.toBytesLE()) })]
        );
        await inst.updateCounters(this.rpc, signers);

        const ctx = new ClientTransaction({ instructions: [inst] });
        ctx.signWith(signers);

        await this.rpc.sendTransactionAndWait(ctx, wait);
    }

    /**
     * Update the data of this instance
     */
    async update(): Promise<CoinInstance> {
        const p = await this.rpc.getProof(this.iid);
        if (!p.matches()) {
            throw new Error('fail to get a matching proof');
        }

        this.coin = Coin.decode(p.value);
        return this;
    }

    static async create(bc: ByzCoinRPC, iid: Buffer, signers: Signer[], type: Buffer = SpawnerCoin): Promise<CoinInstance> {
        const inst = Instruction.createSpawn(
            iid,
            CoinInstance.contractID,
            [new Argument({ name: "type", value: type })]
        );
        await inst.updateCounters(bc, signers);

        const ctx = new ClientTransaction({ instructions: [inst] });
        ctx.signWith(signers);

        await bc.sendTransactionAndWait(ctx, 10);

        const coinIID = inst.deriveId();
        const p = await bc.getProof(coinIID);
        if (!p.matches()) {
            throw new Error('fail to get a matching proof');
        }

        return new CoinInstance(bc, coinIID, Coin.decode(p.value));
    }

    static fromProof(bc: ByzCoinRPC, p: Proof): CoinInstance {
        if (!p.matches()) {
            throw new Error('fail to get a matching proof');
        }

        return new CoinInstance(bc, p.key, Coin.decode(p.value));
    }

    /**
     * Initializes using an existing coinInstance from ByzCoin
     * @param bc
     * @param iid
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: Buffer): Promise<CoinInstance> {
        return CoinInstance.fromProof(bc, await bc.getProof(iid));
    }
}

export class Coin extends Message<Coin> {
    name: Buffer;
    value: Long;

    toBytes(): Buffer {
        return Buffer.from(Coin.encode(this).finish());
    }
}

registerMessage('byzcoin.Coin', Coin);
