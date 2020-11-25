import { randomBytes } from "crypto-browserify";
import { ec } from "elliptic";
import Keccak from "keccak";
import Long from "long";

import { Argument, ClientTransaction, Instance, InstanceID, Instruction } from "../byzcoin";
import ByzCoinRPC from "../byzcoin/byzcoin-rpc";
import Signer from "../darc/signer";
import Log from "../log";

import { BEvmRPC } from "./bevm-rpc";

import { Transaction } from "ethereumjs-tx";
import * as rlp from "rlp";

import { Interface, Result } from "@ethersproject/abi";

/**
 * Ethereum account
 */
export class EvmAccount {
    static EC = new ec("secp256k1");

    static deserialize(obj: any): EvmAccount {
        return new EvmAccount(obj.name, obj.privateKey, obj.nonce);
    }

    private static computeAddress(key: ec.KeyPair): Buffer {
        // Marshal public key to binary
        const pubBytes = Buffer.from(key.getPublic("hex"), "hex");

        const h = new Keccak("keccak256");
        h.update(pubBytes.slice(1));

        const address = h.digest().slice(12);
        Log.lvl3("Computed account address", address.toString("hex"));

        return address;
    }

    readonly address: Buffer;
    protected key: ec.KeyPair;
    private _nonce: number;

    get nonce(): number {
        return this._nonce;
    }

    /**
     * Create a new EVM account
     *
     * @param name      Account name
     * @param privKey   Private key for the account (32 bytes); if not
     *                  specified, a new one is generated randomly
     * @param nonce     Initial nonce for the account; should normally not be specified
     */
    constructor(readonly name: string, privKey?: Buffer, nonce: number = 0) {
        if (privKey === undefined) {
            privKey = randomBytes(32);
        }
        this.key = EvmAccount.EC.keyFromPrivate(privKey);

        this.address = EvmAccount.computeAddress(this.key);
        this._nonce = nonce;
    }

    sign(tx: Transaction) {
        const privKey = Buffer.from(this.key.getPrivate("hex"), "hex");
        tx.sign(privKey);
    }

    incNonce() {
        this._nonce += 1;
    }

    serialize(): object {
        return {
            name: this.name,
            nonce: this.nonce,
            privateKey: this.key.getPrivate("hex"),
        };
    }
}

/**
 * Ethereum smart contract
 */
export class EvmContract {
    static deserialize(obj: any): EvmContract {
        const addresses = obj.addresses.map((elem: string) => Buffer.from(elem, "hex"));
        const bytecode = Buffer.from(obj.bytecode, "hex");

        const contract = new EvmContract(obj.name, bytecode, obj.abi, addresses);

        return contract;
    }

    private static computeAddress(data: Buffer, nonce: number): Buffer {
        const buf = rlp.encode([data, nonce]);

        const h = new Keccak("keccak256");
        h.update(buf);

        const address = h.digest().slice(12);
        Log.lvl3("Computed contract address", address.toString("hex"));

        return address;
    }

    readonly transactions: string[];
    readonly viewMethods: string[];
    private methodAbi: Map<string, any>;

    /**
     * Create a new EVM contract
     *
     * @param name      Contract name
     * @param bytecode  Contract bytecode; pass `undefined` if the contract
     *                  does not need deploying
     * @param abiJson   Contract ABI (JSON-encoded)
     * @param addresses Array of existing (already deployed) contract
     *                  instances; if not provided, the contract will need to
     *                  deploy instances before exeuting transactions or
     *                  view method calls
     */
    constructor(readonly name: string,
                private bytecode: Buffer = Buffer.from(""),
                private abiJson: string,
                readonly addresses: Buffer[] = []) {
        this.methodAbi = new Map();
        this.transactions = [];
        this.viewMethods = [];

        const abiObj = JSON.parse(abiJson);
        abiObj.forEach((item: any) => {
                switch (item.type) {
                    case "constructor": {
                        this.methodAbi.set("", item);
                        break;
                    }

                    case "function": {
                        this.methodAbi.set(item.name, item);

                        if (item.stateMutability === "view") {
                            this.viewMethods.push(item.name);
                        } else {
                            this.transactions.push(item.name);
                        }

                        break;
                    }
                }
            });
    }

    getMethodAbi(method: string): any {
        const entry = this.methodAbi.get(method);
        if (entry === undefined) {
            throw Error(`Method "${method}" does not exist in the ABI`);
        }

        return entry;
    }

    encodeDeployData(...args: any[]): Buffer {
        if (this.bytecode.length === 0) {
            throw Error("This contract cannot be deployed (no provided bytecode)");
        }

        const abiIface = new Interface(this.abiJson);
        const encodedArgs = abiIface.encodeDeploy(args);
        const encodedArgsBuf = Buffer.from(encodedArgs.substring(2), "hex");
        const callDataBuf = Buffer.concat([this.bytecode, encodedArgsBuf]);

        return callDataBuf;
    }

    encodeCallData(method: string, ...args: any[]): Buffer {
        const abiIface = new Interface(this.abiJson);
        const fragment = this.getMethodFragment(method);
        const callData = abiIface.encodeFunctionData(fragment, args);
        const callDataBuf = Buffer.from(callData.substring(2), "hex");

        return callDataBuf;
    }

    decodeCallResult(method: string, result: Buffer): Result {
        const abiIface = new Interface(this.abiJson);
        const fragment = this.getMethodFragment(method);
        const decodedResult = abiIface.decodeFunctionResult(fragment, result);

        return decodedResult;
    }

    /**
     * Add a new instance to this conctract, with the address determine by the
     * provided account.
     *
     * @param account Account that spawned the instance
     */
    addNewInstance(account: EvmAccount) {
        const newAddress = EvmContract.computeAddress(account.address, account.nonce);
        this.addresses.push(newAddress);
    }

    serialize(): object {
        return {
            abi: this.abiJson,
            addresses: this.addresses.map((address) => address.toString("hex")),
            bytecode: this.bytecode.toString("hex"),
            name: this.name,
        };
    }

    private getMethodFragment(method: string): string {
        const entry = this.getMethodAbi(method);

        const types = entry.inputs.map((arg: any) => arg.type);
        const fragment = `${method}(${types.join(",")})`;

        return fragment;
    }
}

// Number of WEIs in one ETHER
export const WEI_PER_ETHER = Long.fromString("1000000000000000000");

/**
 * BEvm client
 */
export class BEvmInstance extends Instance {
    static readonly contractID = "bevm";

    static readonly commandTransaction = "transaction";
    static readonly argumentTx = "txRlp";

    static readonly commandCredit = "credit";
    static readonly argumentAddress = "address";
    static readonly argumentAmount = "amount";

    /**
     * Spawn a new BEvm instance
     *
     * @param bc        ByzCoin RPC to use
     * @param darcID    DARC instance ID
     * @param signers   List of signers for the ByzCoin transaction
     *
     * @return New BEvm instance
     */
    static async spawn(bc: ByzCoinRPC, darcID: InstanceID, signers: Signer[]):
        Promise<BEvmInstance> {
        const inst = Instruction.createSpawn(
            darcID,
            BEvmInstance.contractID,
            [],
        );

        const ctx = ClientTransaction.make(bc.getProtocolVersion(), inst);
        await ctx.updateCountersAndSign(bc, [signers]);

        await bc.sendTransactionAndWait(ctx);

        return BEvmInstance.fromByzcoin(bc, ctx.instructions[0].deriveId(), 2);
    }

    /**
     * Retrieve an existing BEvm instance
     *
     * @param bc    ByzCoin RPC to use
     * @param iid   BEvm instance ID
     *
     * @returns BEvm instance
     */
    static async fromByzcoin(bc: ByzCoinRPC, iid: InstanceID,
                             waitMatch: number = 0,
                             interval: number = 1000): Promise<BEvmInstance> {
        const instance = await Instance.fromByzcoin(bc, iid, waitMatch, interval);

        return new BEvmInstance(bc, instance);
    }

    private bevmRPC: BEvmRPC;

    constructor(private byzcoinRPC: ByzCoinRPC, inst: Instance) {
        super(inst);

        if (inst.contractID.toString() !== BEvmInstance.contractID) {
            throw new Error(`mismatch contract name: ${inst.contractID} vs ${BEvmInstance.contractID}`);
        }

        // Use first cothority server for service
        const bevmRPC = new BEvmRPC(byzcoinRPC.getConfig().roster.list[0]);
        bevmRPC.setTimeout(1000);

        this.bevmRPC = bevmRPC;
    }

    /**
     * Deploy an EVM smart contract
     *
     * @param signers   ByzCoin identities signing the ByzCoin transaction
     * @param gasLimit  Gas limit for the transaction
     * @param gasPrice  Gas price for the transaction
     * @param amount    Amount transferred during the transaction
     * @param account   EVM account
     * @param contract  EVM contract to deploy
     * @param args      Arguments for the smart contract constructor
     * @param wait      Number of blocks to wait for the ByzCoin transaction to be included
     *
     * The `args` are passed as an array of values, one per argument. The types
     * of the arguments must match the types in the constructor ABI. In
     * particular, for large number types (e.g. uint256), the argument can be
     * passed as a number, string or BN value.
     */
    async deploy(signers: Signer[],
                 gasLimit: number,
                 gasPrice: number,
                 amount: number,
                 account: EvmAccount,
                 contract: EvmContract,
                 args: any[] = [],
                 wait?: number) {
        const callData = contract.encodeDeployData(...args);

        const ethTx = new Transaction({
            data: callData,
            gasLimit,
            gasPrice,
            nonce: account.nonce,
            value: amount,
        });
        account.sign(ethTx);

        await this.invoke(
            BEvmInstance.commandTransaction, [
                new Argument({name: BEvmInstance.argumentTx,
                             value: ethTx.serialize()}),
            ],
            signers, wait);

        contract.addNewInstance(account);
        account.incNonce();
    }

    /**
     * Execute an EVM transaction on a deployed smart contract (R/W method)
     *
     * @param signers       ByzCoin identities signing the ByzCoin transaction
     * @param gasLimit      Gas limit for the transaction
     * @param gasPrice      Gas price for the transaction
     * @param amount        Amount transferred during the transaction
     * @param account       EVM account
     * @param contract      EVM contract
     * @param instanceIndex Index of the deployed smart contract instance
     * @param method        Name of the method to execute
     * @param args          Arguments for the smart contract method
     * @param wait          Number of blocks to wait for the ByzCoin transaction to be included
     *
     * See `deploy()` for a description of `args`.
     */
    async transaction(signers: Signer[],
                      gasLimit: number,
                      gasPrice: number,
                      amount: number,
                      account: EvmAccount,
                      contract: EvmContract,
                      instanceIndex: number,
                      method: string,
                      args: any[] = [],
                      wait?: number) {
        if (instanceIndex >= contract.addresses.length) {
            throw Error(`instance ${instanceIndex} does not exist`);
        }

        const callData = contract.encodeCallData(method, ...args);

        const ethTx = new Transaction({
            data: callData,
            gasLimit,
            gasPrice,
            nonce: account.nonce,
            to: contract.addresses[instanceIndex],
            value: amount,
        });
        account.sign(ethTx);

        await this.invoke(
            BEvmInstance.commandTransaction, [
                new Argument({name: BEvmInstance.argumentTx,
                             value: ethTx.serialize()}),
            ],
            signers, wait);

        account.incNonce();
    }

    /**
     * Execute a view method on a deployed smart contract (read-only method)
     *
     * A view method is executed on a conode on behalf of the client. This
     * conode does not necessarily belong to the ByzCoin cothority. Therefore,
     * the actual cothority configuration, the BEvm ID as well as the ByzCoin
     * ID that contains it must be provided.
     *
     * @param account           EVM account
     * @param contract          EVM contract
     * @param instanceIndex     Index of the deployed smart contract instance
     * @param method            Name of the view method to execute
     * @param args              Arguments for the smart contract method
     *
     * @return Result of the view method execution, as an array of values. The
     * types of the values will match the return values in the view method ABI.
     * In particular, values for large number types (e.g. uint256) will be BNs.
     *
     * See `deploy()` for a description of `args`.
     */
    async call(account: EvmAccount,
               contract: EvmContract,
               instanceIndex: number,
               method: string,
               args: any[] = []): Promise<Result> {
        if (instanceIndex >= contract.addresses.length) {
            throw Error(`instance ${instanceIndex} does not exist`);
        }

        const callData = contract.encodeCallData(method, ...args);

        const response = await this.bevmRPC.viewCall(
            this.byzcoinRPC.genesisID,
            this.id,
            account.address,
            contract.addresses[instanceIndex],
            callData);

        const decodedResult = contract.decodeCallResult(method, response.result);

        return decodedResult;
    }

    /**
     * Credit an EVM account by the specified amount
     *
     * @param signers   ByzCoin identities signing the ByzCoin transaction
     * @param account   EVM account
     * @param amount    Amount to credit on the account
     * @param wait      Number of blocks to wait for the ByzCoin transaction to be included
     */
    async creditAccount(signers: Signer[],
                        account: EvmAccount,
                        amount: Long,
                        wait?: number) {
        const amountBuf = Buffer.from(amount.toBytesBE());

        await this.invoke(
            BEvmInstance.commandCredit,
            [
                new Argument({name: BEvmInstance.argumentAddress,
                             value: account.address}),
                new Argument({name: BEvmInstance.argumentAmount,
                             value: amountBuf}),
            ],
            signers,
            wait);
    }

    private async invoke(command: string,
                         args: Argument[],
                         signers: Signer[],
                         wait?: number) {
        const ctx = ClientTransaction.make(
            this.byzcoinRPC.getProtocolVersion(),
            Instruction.createInvoke(
                this.id, BEvmInstance.contractID, command, args,
            ));

        await ctx.updateCountersAndSign(this.byzcoinRPC, [signers]);
        await this.byzcoinRPC.sendTransactionAndWait(ctx, wait);
    }
}
