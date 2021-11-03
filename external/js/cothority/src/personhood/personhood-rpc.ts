import { ServerIdentity, WebSocketConnection } from "../network";
import { EmailRecover, EmailRecoverReply, EmailSignup, EmailSignupReply } from "./proto";

/**
 * RPC to talk with the personhood service. This is only a partial implementation of the
 * personhood service to enable email signups.
 */
export default class PersonhoodRPC {
    static serviceName = "Personhood";

    private conn: WebSocketConnection;
    private timeout: number;

    constructor(node: ServerIdentity) {
        this.timeout = 10 * 1000;
        this.conn = new WebSocketConnection(node.getWebSocketAddress(), PersonhoodRPC.serviceName);
    }

    /**
     * Set a new timeout value for the next requests
     * @param value Timeout in ms
     */
    setTimeout(value: number): void {
        this.timeout = value;
    }

    /**
     * Signs up the new user with the given email and alias. If the new user doesn't exist yet, it will be created,
     * and an email with signup instructions is sent to the given address.
     * Additionally, the id of the user will be added to the configured DARC.
     * @param email of the new user
     * @param alias of the new user
     */
    async signup(email: string, alias: string): Promise<EmailSignupReply> {
        return this.conn.send(new EmailSignup({email, alias}), EmailSignupReply);
    }

    /**
     * Sends the given email for recovery to the service. The service searches for a user with the given email, but
     * only in all created users with 'signup'. If exactly one match is found, a recovery device is created, and the
     * instructions to recover that device are sent to the email given.
     * @param email of an existing user.
     */
    async recover(email: string): Promise<EmailRecoverReply> {
        return this.conn.send(new EmailRecover({email}), EmailRecoverReply);
    }
}
