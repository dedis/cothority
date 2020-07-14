import Log from "../log";
import { ServerIdentity, WebSocketConnection } from "../network";
import {
    CallRequest,
    CallResponse,
} from "./proto";

/**
 * BEvm service
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
     *
     * @param value Timeout in [ms]
     */
    setTimeout(value: number): void {
        this.timeout = value;
    }

    /**
     * Execute a view method (read-only)
     *
     * @param byzcoinId         ByzCoin ID
     * @param serverConfig      Cothority server config in TOML
     * @param bevmInstanceId    BEvm instance ID
     * @param accountAddress    Address of the EVM account
     * @param contractAddress   Address of the smart contract
     * @param callData          ABI-packed call arguments
     *
     * @return Result of the view method execution
     */
    async performCall(byzcoinId: Buffer,
                      serverConfig: string,
                      bevmInstanceId: Buffer,
                      accountAddress: Buffer,
                      contractAddress: Buffer,
                      callData: Buffer):
                          Promise<CallResponse> {
        this.conn.setTimeout(this.timeout);

        Log.lvl3("Sending BEvm call request...");

        const msg = new CallRequest({
                accountAddress,
                bevmInstanceId,
                byzcoinId,
                callData,
                contractAddress,
                serverConfig,
            });

        return this.conn.send(msg, CallResponse);
    }
}
