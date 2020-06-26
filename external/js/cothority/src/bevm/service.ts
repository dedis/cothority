import Log from "../log";
import { ServerIdentity, WebSocketConnection } from "../network";
import {
    CallRequest,
    CallResponse,
    DeployRequest,
    TransactionFinalizationRequest,
    TransactionHashResponse,
    TransactionRequest,
    TransactionResponse,
} from "./proto";

/**
 * Client to talk to the BEvm service
 */
export class BEvmService {
    static serviceName = "BEvm";

    private conn: WebSocketConnection;
    private timeout: number;

    constructor(srvid: ServerIdentity) {
        this.timeout = 60 * 1000; // 60 seconds
        this.conn = new WebSocketConnection(srvid.getWebSocketAddress(),
                                            BEvmService.serviceName);
    }

    /**
     * Set a new timeout value for future requests.
     * @param value Timeout in [ms]
     */
    setTimeout(value: number): void {
        this.timeout = value;
    }

    async prepareDeployTx(gasLimit: number,
                          gasPrice: number,
                          amount: number,
                          nonce: number,
                          bytecode: Buffer,
                          abi: string,
                          args: string[]): Promise<TransactionHashResponse> {
        this.conn.setTimeout(this.timeout);

        Log.lvl2("Sending BEvm deploy contract request...");

        const msg = new DeployRequest({
            abi,
            amount,
            args,
            bytecode,
            gasLimit,
            gasPrice,
            nonce,
        });

        return this.conn.send(msg, TransactionHashResponse);
    }

    async prepareTransactionTx(gasLimit: number,
                               gasPrice: number,
                               amount: number,
                               contractAddress: Buffer,
                               nonce: number,
                               abi: string,
                               method: string,
                               args: string[]): Promise<TransactionHashResponse> {
        this.conn.setTimeout(this.timeout);

        Log.lvl2("Sending BEvm transaction execution request...");

        const msg = new TransactionRequest({
                abi,
                amount,
                args,
                contractAddress,
                gasLimit,
                gasPrice,
                method,
                nonce,
            });

        return this.conn.send(msg, TransactionHashResponse);
    }

    async finalizeTx(transaction: Buffer,
                     signature: Buffer): Promise<TransactionResponse> {
        this.conn.setTimeout(this.timeout);

        Log.lvl2("Sending BEvm transaction finalization request...");

        const msg = new TransactionFinalizationRequest({
                signature,
                transaction,
            });

        return this.conn.send(msg, TransactionResponse);
    }

    async performCall(blockId: Buffer,
                      serverConfig: string,
                      bevmInstanceId: Buffer,
                      accountAddress: Buffer,
                      contractAddress: Buffer,
                      abi: string,
                      method: string,
                      args: string[]):
                          Promise<CallResponse> {
        this.conn.setTimeout(this.timeout);

        Log.lvl2("Sending BEvm call request...");

        const msg = new CallRequest({
                abi,
                accountAddress,
                args,
                bevmInstanceId,
                blockId,
                contractAddress,
                method,
                serverConfig,
            });

        return this.conn.send(msg, CallResponse);
    }
}
