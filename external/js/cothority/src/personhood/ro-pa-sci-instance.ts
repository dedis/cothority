import { Message, Properties } from "protobufjs/light";

import { LongTermSecret } from "../calypso";

import { curve } from "@dedis/kyber";
import ByzCoinRPC from "../byzcoin/byzcoin-rpc";
import ClientTransaction, { Argument, Instruction } from "../byzcoin/client-transaction";
import CoinInstance, { Coin } from "../byzcoin/contracts/coin-instance";
import Instance, { InstanceID } from "../byzcoin/instance";
import Signer from "../darc/signer";
import { EMPTY_BUFFER, registerMessage } from "../protobuf";

const curve25519 = curve.newCurve("edwards25519");

export default class RoPaSciInstance extends Instance {

    get stake(): Coin {
        return this.struct.stake;
    }

    get playerChoice(): number {
        return this.struct.firstPlayer;
    }

    /**
     * Getter for the second player ID
     * @returns id as a buffer
     */
    get adversaryID(): Buffer {
        return this.struct.secondPlayerAccount;
    }

    /**
     * Getter for the second player choice
     * @returns the choice as a number
     */
    get adversaryChoice(): number {
        return this.struct.secondPlayer;
    }

    static readonly contractID = "ropasci";

    static fromObject(rpc: ByzCoinRPC, obj: any) {
        const inst = Instance.fromBytes(obj.spawnerInstance);
        const rps = new RoPaSciInstance(rpc, inst);
        rps.fillUp = obj.fillUp;
        rps.firstMove = obj.firstMove;
        return rps;
    }

    /**
     * Fetch the proof for the given instance and create a
     * RoPaSciInstance from it
     *
     * @param bc    The ByzCoinRPC to use
     * @param iid   The instance ID
     * @param waitMatch how many times to wait for a match - useful if its called just after an addTransactionAndWait.
     * @param interval how long to wait between two attempts in waitMatch.
     * @returns the new instance
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID, waitMatch: number = 0, interval: number = 1000):
        Promise<RoPaSciInstance> {
        return new RoPaSciInstance(bc, await Instance.fromByzcoin(bc, iid, waitMatch, interval));
    }
    struct: RoPaSciStruct;
    private fillUp: Buffer | undefined;
    private firstMove: number | undefined;

    constructor(private rpc: ByzCoinRPC, inst: Instance) {
        super(inst);
        if (inst.contractID.toString() !== RoPaSciInstance.contractID) {
            throw new Error(`mismatch contract name: ${inst.contractID} vs ${RoPaSciInstance.contractID}`);
        }

        this.struct = RoPaSciStruct.decode(this.data);
    }

    /**
     * Update the instance data
     *
     * @param choice The choice of the first player
     * @param fillup The fillup of the first player
     */
    setChoice(choice: number, fillup: Buffer) {
        this.firstMove = choice;
        this.fillUp = fillup;
    }

    /**
     * Returns the firstMove and the fillUp values.
     */
    getChoice(): [number, Buffer | undefined] {
        return [this.firstMove, this.fillUp ? Buffer.from(this.fillUp) : undefined];
    }

    /**
     * Check if both players have played their moves
     *
     * @returns true when both have played, false otherwise
     */
    isDone(): boolean {
        return this.struct.secondPlayer >= 0;
    }

    /**
     * returns true if there is a CalypsoWrite instance stored.
     */
    isCalypso(): boolean {
        return !this.struct.calypsoWrite.equals(Buffer.alloc(32));
    }

    /**
     * ourGame returns if this game belongs to the given coin.
     * @param coinID
     */
    ourGame(coinID: InstanceID): boolean {
        const player1 = this.struct.firstPlayerAccount;
        if (player1 !== undefined && !player1.equals(Buffer.alloc(32))) {
            return player1.equals(coinID);
        }
        return this.getChoice()[1] !== undefined;
    }

    /**
     * Play the adversary move
     *
     * @param coin      The CoinInstance of the second player
     * @param signer    Signer for the transaction
     * @param choice    The choice of the second player
     * @returns a promise that resolves on success, or rejects with the error
     */
    async second(coin: CoinInstance, signer: Signer, choice: number, lts?: LongTermSecret): Promise<void> {
        if (!coin.name.equals(this.struct.stake.name)) {
            throw new Error("not correct coin-type for player 2");
        }
        if (coin.value.lessThan(this.struct.stake.value)) {
            throw new Error("don't have enough coins to match stake");
        }

        const args = [
            new Argument({name: "account", value: coin.id}),
            new Argument({name: "choice", value: Buffer.from([choice % 3])}),
        ];
        const priv = curve25519.scalar().pick();
        const pub = curve25519.point().mul(priv);
        if (this.isCalypso()) {
            if (lts === undefined) {
                throw new Error("need LTS for calypso-ropascis");
            }
            args.push(new Argument({name: "public", value: pub.marshalBinary()}));
        }

        const ctx = ClientTransaction.make(
            this.rpc.getProtocolVersion(),
            Instruction.createInvoke(
                coin.id,
                CoinInstance.contractID,
                CoinInstance.commandFetch,
                [
                    new Argument({
                        name: CoinInstance.argumentCoins,
                        value: Buffer.from(this.struct.stake.value.toBytesLE()),
                    }),
                ],
            ),
            Instruction.createInvoke(
                this.id,
                RoPaSciInstance.contractID,
                "second",
                args,
            ),
        );
        await ctx.updateCountersAndSign(this.rpc, [[signer], []]);

        await this.rpc.sendTransactionAndWait(ctx);
        await this.update();
        if (this.isCalypso()) {
            const dreply = await lts.reencryptKey(await this.rpc.getProof(this.struct.calypsoWrite),
                await this.rpc.getProof(this.struct.calypsoRead));
            const preHash = await dreply.decrypt(priv);
            this.firstMove = preHash[0];
            this.fillUp = Buffer.allocUnsafe(31);
            preHash.slice(1).copy(this.fillUp);
            await this.confirm(coin);
        }
    }

    /**
     * Reveal the move of the first player
     *
     * @param coin The CoinInstance of the first player
     * @returns a promise that resolves on success, or rejects
     * with the error
     */
    async confirm(coin: CoinInstance): Promise<void> {
        if (!coin.name.equals(this.struct.stake.name)) {
            throw new Error("not correct coin-type for player 1");
        }

        const preHash = Buffer.alloc(this.fillUp.length + 1, 0);
        preHash[0] = this.firstMove % 3;
        this.fillUp.copy(preHash, 1);
        const ctx = ClientTransaction.make(this.rpc.getProtocolVersion(), Instruction.createInvoke(
            this.id,
            RoPaSciInstance.contractID,
            "confirm",
            [
                new Argument({name: "prehash", value: preHash}),
                new Argument({name: "account", value: coin.id}),
            ],
        ));
        await this.rpc.sendTransactionAndWait(ctx);
        await this.update();
    }

    /**
     * Update the state of the instance
     *
     * @returns a promise that resolves with the updated instance,
     * or rejects with the error
     */
    async update(): Promise<RoPaSciInstance> {
        const proof = await this.rpc.getProofFromLatest(this.id);
        if (!proof.exists(this.id)) {
            throw new Error("fail to get a matching proof");
        }

        const inst = Instance.fromProof(this.id, proof);
        this.data = inst.data;
        this.struct = RoPaSciStruct.decode(this.data);
        return this;
    }

    toObject(): any {
        return{
            fillUp: this.fillUp,
            firstMove: this.firstMove,
            instance: this.toBytes(),
        };
    }
}

/**
 * Data hold by a rock-paper-scissors instance
 */
export class RoPaSciStruct extends Message<RoPaSciStruct> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("personhood.RoPaSciStruct", RoPaSciStruct);
    }

    readonly description: string;
    readonly stake: Coin;
    readonly firstPlayerHash: Buffer;
    readonly firstPlayer: number;
    readonly secondPlayer: number;
    readonly secondPlayerAccount: Buffer;
    readonly firstPlayerAccount: Buffer | undefined;
    readonly calypsoWrite: Buffer;
    readonly calypsoRead: Buffer;

    constructor(props?: Properties<RoPaSciStruct>) {
        super(props);

        this.firstPlayerHash = Buffer.from(this.firstPlayerHash || EMPTY_BUFFER);
        this.secondPlayerAccount = Buffer.from(this.secondPlayerAccount || EMPTY_BUFFER);

        Object.defineProperty(this, "firstplayer", {
            get(): number {
                return this.firstPlayer;
            },
            set(value: number) {
                this.firstPlayer = value;
            },
        });

        Object.defineProperty(this, "firstplayerhash", {
            get(): Buffer {
                return this.firstPlayerHash;
            },
            set(value: Buffer) {
                this.firstPlayerHash = value;
            },
        });

        Object.defineProperty(this, "secondplayer", {
            get(): number {
                return this.secondPlayer;
            },
            set(value: number) {
                this.secondPlayer = value;
            },
        });

        Object.defineProperty(this, "secondplayeraccount", {
            get(): Buffer {
                return this.secondPlayerAccount;
            },
            set(value: Buffer) {
                this.secondPlayerAccount = value;
            },
        });

        Object.defineProperty(this, "firstplayeraccount", {
            get(): Buffer {
                return this.firstPlayerAccount;
            },
            set(value: Buffer) {
                this.firstPlayerAccount = value;
            },
        });

        Object.defineProperty(this, "calypsowrite", {
            get(): Buffer {
                return this.calypsoWrite;
            },
            set(value: Buffer) {
                this.calypsoWrite = value;
            },
        });

        Object.defineProperty(this, "calypsoread", {
            get(): Buffer {
                return this.calypsoRead;
            },
            set(value: Buffer) {
                this.calypsoRead = value;
            },
        });
    }

    /**
     * Helper to encode the struct using protobuf
     *
     * @returns the data as a buffer
     */
    toBytes(): Buffer {
        return Buffer.from(RoPaSciStruct.encode(this).finish());
    }
}

RoPaSciStruct.register();
