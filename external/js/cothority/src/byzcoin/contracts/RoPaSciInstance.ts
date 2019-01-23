import {BasicInstance} from "../../byzcoin/contracts/Instance";
import {ByzCoinRPC} from "../../byzcoin/ByzCoinRPC";
import {Proof} from "../../byzcoin/Proof";
import {Coin, CoinInstance} from "../../byzcoin/contracts/CoinInstance";
import {Argument, ClientTransaction, InstanceID, Instruction} from "../../byzcoin/ClientTransaction";
import {objToProto, Root} from "../../protobuf/Root";
import {Signer} from "../../darc/Signer";
import {Buffer} from "buffer";
import {Log} from "~/lib/Log";
import {Spawner} from "../../byzcoin/contracts/SpawnerInstance";

const crypto = require("crypto-browserify");

export class RoPaSciInstance extends BasicInstance {
    static readonly contractID = "ropasci";

    public roPaSciStruct: RoPaSciStruct;
    public fillUp: Buffer = null;
    public firstMove: number = -1;

    constructor(public bc: ByzCoinRPC, p: Proof | any = null) {
        super(bc, RoPaSciInstance.contractID, p);
        this.roPaSciStruct = RoPaSciStruct.fromProto(this.data);
        if (p && !p.matchContract && p.fillup){
            this.fillUp = Buffer.from(p.fillup);
            this.firstMove = p.firstmove;
        }
    }

    async second(coin: CoinInstance, signer: Signer, choice: number): Promise<any> {
        if (!coin.coin.name.equals(this.roPaSciStruct.stake.name)) {
            return Promise.reject("not correct coin-type for player 2");
        }
        if (coin.coin.value.lessThan(this.roPaSciStruct.stake.value)) {
            return Promise.reject("don't have enough coins to match stake");
        }
        let ctx = new ClientTransaction([
            Instruction.createInvoke(coin.iid,
                "fetch", [
                    new Argument("coins", Buffer.from(this.roPaSciStruct.stake.value.toBytesLE()))
                ]),
            Instruction.createInvoke(this.iid,
                "second", [
                    new Argument("account", coin.iid.iid),
                    new Argument("choice", Buffer.from([choice % 3]))
                ])
        ]);
        await ctx.signBy([[signer], []], this.bc);
        await this.bc.sendTransactionAndWait(ctx);
    }

    async confirm(coin: CoinInstance) {
        if (!coin.coin.name.equals(this.roPaSciStruct.stake.name)) {
            return Promise.reject("not correct coin-type for player 1");
        }
        let preHash = Buffer.alloc(32);
        preHash[0] = this.firstMove % 3;
        this.fillUp.copy(preHash, 1);
        let ctx = new ClientTransaction([
            Instruction.createInvoke(this.iid,
                "confirm", [
                    new Argument("prehash", preHash),
                    new Argument("account", coin.iid.iid),
                ])
        ]);
        await this.bc.sendTransactionAndWait(ctx);
    }

    async update(): Promise<RoPaSciInstance> {
        let proof = await this.bc.getProof(this.iid);
        this.roPaSciStruct = RoPaSciStruct.fromProto(proof.value);
        return this;
    }

    isDone(): boolean {
        return this.roPaSciStruct.secondPlayer >= 0;
    }

    toObject(): object {
        let o = super.toObject();
        o.fillup = this.fillUp;
        o.firstmove = this.firstMove;
        return o;
    }

    static fromObject(bc: ByzCoinRPC, obj: any): RoPaSciInstance {
        return new RoPaSciInstance(bc, obj);
    }

    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<RoPaSciInstance> {
        return new RoPaSciInstance(bc, await bc.getProof(iid));
    }
}

export class RoPaSciStruct {
    static readonly protoName = "personhood.RoPaSciStruct";

    constructor(public description, public stake: Coin, public firstPlayerHash: Buffer, public firstPlayer: number,
                public secondPlayer: number, public secondPlayerAccount: InstanceID) {
    }

    toObject(): object {
        return {
            description: this.description,
            stake: this.stake.toObject(),
            firstplayerhash: this.firstPlayerHash,
            firstplayer: this.firstPlayer,
            secondplayer: this.secondPlayer,
            secondplayeraccount: this.secondPlayerAccount,
        }
    }

    toProto(): Buffer {
        return objToProto(this.toObject(), RoPaSciStruct.protoName);
    }

    static fromProto(buf: Buffer): RoPaSciStruct {
        let rps = Root.lookup(RoPaSciStruct.protoName).decode(buf);
        return new RoPaSciStruct(rps.description, new Coin(rps.stake), Buffer.from(rps.firstplayerhash),
            rps.firstplayer, rps.secondplayer, InstanceID.fromObjectBuffer(rps.secondplayeraccount));
    }
}