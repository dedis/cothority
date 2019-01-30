import Proof from "../proof";
import ClientTransaction, {Argument, Instruction} from "../client-transaction";
import Signer from "../../darc/signer";
import ByzCoinRPC from "../byzcoin-rpc";
import Instance, { InstanceID } from "../instance";
import CoinInstance, { Coin } from "./coin-instance";
import { Message } from "protobufjs";

export default class RoPaSciInstance {
    static readonly contractID = "ropasci";

    private rpc: ByzCoinRPC;
    private instance: Instance;
    private struct: RoPaSciStruct;
    private fillUp: Buffer;
    private firstMove: number;

    constructor(bc: ByzCoinRPC, inst: Instance) {
        this.rpc = bc;
        this.instance = inst;
        this.struct = RoPaSciStruct.decode(this.instance.data);
    }

    setChoice(choice: number, fillup: Buffer) {
        this.firstMove = choice;
        this.fillUp = fillup;
    }

    async second(coin: CoinInstance, signer: Signer, choice: number): Promise<void> {
        if (!coin.name.equals(this.struct.stake.name)) {
            return Promise.reject("not correct coin-type for player 2");
        }
        if (coin.value.lessThan(this.struct.stake.value)) {
            return Promise.reject("don't have enough coins to match stake");
        }

        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createInvoke(
                    coin.id,
                    CoinInstance.contractID,
                    "fetch",
                    [
                        new Argument({ name: "coins", value: Buffer.from(this.struct.stake.value.toBytesLE()) }),
                    ],
                ),
                Instruction.createInvoke(
                    this.instance.id,
                    RoPaSciInstance.contractID,
                    "second",
                    [
                        new Argument({ name: "account", value: coin.id }),
                        new Argument({ name: "choice", value: Buffer.from([choice % 3]) }),
                    ],
                )
            ]
        });

        await ctx.updateCounters(this.rpc, [signer]);

        ctx.signWith([signer]);
        await this.rpc.sendTransactionAndWait(ctx);
    }

    async confirm(coin: CoinInstance): Promise<void> {
        if (!coin.name.equals(this.struct.stake.name)) {
            throw new Error("not correct coin-type for player 1");
        }

        const preHash = Buffer.alloc(32, 0);
        preHash[0] = this.firstMove % 3;
        this.fillUp.copy(preHash, 1);
        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createInvoke(
                    this.instance.id,
                    RoPaSciInstance.contractID,
                    "confirm",
                    [
                        new Argument({ name: "prehash", value: preHash }),
                        new Argument({ name: "account", value: coin.id }),
                    ],
                )
            ]
        });

        await this.rpc.sendTransactionAndWait(ctx);
    }

    async update(): Promise<RoPaSciInstance> {
        const proof = await this.rpc.getProof(this.instance.id);
        if (!proof.matches()) {
            throw new Error('fail to get a matching proof');
        }

        this.instance = Instance.fromProof(proof);
        this.struct = RoPaSciStruct.decode(this.instance.data);
        return this;
    }

    isDone(): boolean {
        return this.struct.secondPlayer >= 0;
    }

    static fromProof(bc: ByzCoinRPC, p: Proof): RoPaSciInstance {
        if (!p.matches()) {
            throw new Error('fail to get a matching proof');
        }

        return new RoPaSciInstance(bc, Instance.fromProof(p));
    }

    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID): Promise<RoPaSciInstance> {
        return RoPaSciInstance.fromProof(bc, await bc.getProof(iid));
    }
}

export class RoPaSciStruct extends Message<RoPaSciStruct> {
    readonly description: string;
    readonly stake: Coin;
    readonly firstplayerhash: Buffer;
    readonly firstplayer: number;
    readonly secondplayer: number;
    readonly secondplayeraccount: Buffer;

    get firstPlayer(): number {
        return this.firstplayer;
    }

    get secondPlayer(): number {
        return this.secondplayer;
    }

    toBytes(): Buffer {
        return Buffer.from(RoPaSciStruct.encode(this).finish());
    }
}