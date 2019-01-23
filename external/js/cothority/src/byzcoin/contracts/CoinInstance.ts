import {ByzCoinRPC} from "~/lib/cothority/byzcoin/ByzCoinRPC";
import {Instance} from "~/lib/cothority/byzcoin/Instance";
import {Argument, ClientTransaction, InstanceID, Instruction} from "~/lib/cothority/byzcoin/ClientTransaction";
import * as Long from "long";
import {objToProto, Root} from "~/lib/cothority/protobuf/Root";
import {Signer} from "~/lib/cothority/darc/Signer";
import {SpawnerCoin} from "~/lib/cothority/byzcoin/contracts/SpawnerInstance";
import {Log} from "~/lib/Log";
import {Proof} from "~/lib/cothority/byzcoin/Proof";

export class CoinInstance {
    static readonly contractID = "coin";

    constructor(public bc: ByzCoinRPC, public iid: InstanceID, public coin: Coin) {
    }

    /**
     * Transfer a certain amount of coin to another account.
     *
     * @param  coins the amount
     * @param to the destination account (must be a coin contract instance id)
     * @param signers the signers (of the giver account)
     */
    async transfer(coins:Long, to: InstanceID, signers: Signer[]) {
        let args = [];

        args.push(new Argument("coins", new Buffer(coins.toBytesLE())));
        args.push(new Argument("destination", to.iid));

        let inst = Instruction.createInvoke(this.iid, "transfer", args);
        let ctx = new ClientTransaction([inst]);
        await ctx.signBy([signers], this.bc);
        await this.bc.sendTransactionAndWait(ctx, 10);
    }

    async mint(signers: Signer[], amount: Long, wait: number = 5) {
        let inst = Instruction.createInvoke(this.iid,
            "mint",
            [new Argument("coins", Buffer.from(amount.toBytesLE()))]);
        let ctx = new ClientTransaction([inst]);
        await ctx.signBy([signers], this.bc);
        await this.bc.sendTransactionAndWait(ctx, wait);
    }

    /**
     * Update the data of this instance
     */
    async update(): Promise<CoinInstance> {
        let p = await this.bc.getProof(this.iid);
        this.coin = Coin.fromProto(p.value);
        return this;
    }

    static async create(bc: ByzCoinRPC, iid: InstanceID, signers: Signer[], type: InstanceID = SpawnerCoin): Promise<CoinInstance> {
        let inst = Instruction.createSpawn(iid,
            this.contractID,
            [new Argument("type", type.iid)]);
        let ctx = new ClientTransaction([inst]);
        await ctx.signBy([signers], bc);
        await bc.sendTransactionAndWait(ctx, 5);
        let coinIID = new InstanceID(inst.deriveId());
        let p = await bc.getProof(coinIID);
        if (!p.matchContract(CoinInstance.contractID)){
            return Promise.reject("didn't find correct instanceID");
        }
        return new CoinInstance(bc,
            p.requestedIID,
            Coin.fromProto(p.value));
    }

    static fromProof(bc: ByzCoinRPC, p: Proof): CoinInstance{
        p.matchOrFail(CoinInstance.contractID);
        return new CoinInstance(bc, p.requestedIID,
            Coin.fromProto(p.value))
    }

    /**
     * Initializes using an existing coinInstance from ByzCoin
     * @param bc
     * @param iid
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<CoinInstance> {
        return CoinInstance.fromProof(bc, await bc.getProof(iid));
    }
}

export class Coin {
    static readonly protoName = "byzcoin.Coin";

    name: InstanceID;
    value: Long;

    constructor(o: any) {
        this.name = new InstanceID(o.name);
        this.value = o.value;
    }

    toObject(): object {
        return {
            name: this.name.iid,
            value: this.value,
        };
    }

    toProto(): Buffer {
        return objToProto(this.toObject(), Coin.protoName);
    }

    static fromProto(p: Buffer): Coin {
        return new Coin(Root.lookup(Coin.protoName).decode(p));
    }

    static create(name: InstanceID, value: Long): Coin{
        return new Coin({name: name.iid, value: value});
    }
}