import { Point, PointFactory, Scalar } from "@dedis/kyber";
import { Message, Properties } from "protobufjs";
import { Argument, ClientTransaction, InstanceID, Instruction, Proof } from "../byzcoin";
import ByzCoinRPC from "../byzcoin/byzcoin-rpc";
import { Signer } from "../darc";
import Log from "../log";
import { Roster, ServerIdentity } from "../network";
import { IConnection, RosterWSConnection, WebSocketConnection } from "../network/connection";
import { registerMessage } from "../protobuf";
import { DecodeKey, OnChainSecretInstance } from "./calypso-instance";

/**
 * OnChainSecretRPC is used to contact the OnChainSecret service of the cothority.
 * With it you can set up a new long-term onchain-secret, give it a policy to accept
 * new requests, and ask for re-encryption requests.
 */
export class OnChainSecretRPC {
    static serviceID = "Calypso";
    private socket: IConnection;
    private readonly list: ServerIdentity[];

    constructor(public bc: ByzCoinRPC, roster?: Roster) {
        this.socket = new RosterWSConnection(bc.getConfig().roster, OnChainSecretRPC.serviceID);
        if (roster) {
            this.list = roster.list;
        } else {
            this.list = this.bc.getConfig().roster.list;
        }
    }

    // CreateLTS creates a random LTSID that can be used to reference the LTS group
    // created. It first sends a transaction to ByzCoin to spawn a LTS instance,
    // then it asks the Calypso cothority to start the DKG.
    async createLTS(r: Roster, darcID: InstanceID, signers: Signer[]): Promise<CreateLTSReply> {
        const buf = Buffer.from(LtsInstanceInfo.encode(new LtsInstanceInfo({roster: r})).finish());
        const ctx = new ClientTransaction({
            instructions: [
                Instruction.createSpawn(darcID, OnChainSecretInstance.contractID, [
                    new Argument({name: "lts_instance_info", value: buf}),
                ]),
            ],
        });
        await ctx.updateCountersAndSign(this.bc, [signers]);
        await this.bc.sendTransactionAndWait(ctx);
        // Ask for the full proof which is easier to verify.
        const p = await this.bc.getProof(ctx.instructions[0].deriveId());

        return new WebSocketConnection(r.list[0].getWebSocketAddress(), OnChainSecretRPC.serviceID)
            .send(new CreateLTS({proof: p}), CreateLTSReply);
    }

    // authorize adds a ByzCoinID to the list of authorized IDs for each
    // server in the roster. The authorize endpoint refuses requests
    // that do not come from localhost for security reasons.
    //
    // It should be called by the administrator at the beginning, before any other
    // API calls are made. A ByzCoinID that is not authorized will not be allowed to
    // call the other APIs.
    async authorize(who: ServerIdentity, bcid: InstanceID): Promise<AuthorizeReply> {
        const sock = new WebSocketConnection(who.getWebSocketAddress(), OnChainSecretRPC.serviceID);
        return sock.send(new Authorize({byzcoinid: bcid}), AuthorizeReply);
    }

    /**
     * authorizeRoster is a convenience method that authorizes all nodes in the bc-roster
     * to create new LTS. For this to work, the nodes must have been started with
     * COTHORITY_ALLOW_INSECURE_ADMIN=true
     *
     * @param roster if given, this roster is used instead of the bc-roster
     */
    async authorizeRoster(roster?: Roster) {
        if (!roster) {
            roster = this.bc.getConfig().roster;
        }
        for (const node of roster.list) {
            await this.authorize(node, this.bc.genesisID);
        }
    }

    // reencryptKey takes as input Read- and Write- Proofs. It verifies that
    // the read/write requests match and then re-encrypts the secret
    // given the public key information of the reader.
    async reencryptKey(write: Proof, read: Proof): Promise<DecryptKeyReply> {
        const sock = new WebSocketConnection(this.list[0].getWebSocketAddress(), OnChainSecretRPC.serviceID);
        return sock.send(new DecryptKey({read, write}), DecryptKeyReply);
    }
}

/**
 * LongTermSecret extends the OnChainSecretRPC and also holds the id and the X.
 */
export class LongTermSecret extends OnChainSecretRPC {

    /**
     * spawn creates a new longtermsecret by spawning a longTermSecret instance, and then performing
     * a DKG using the full roster of bc.
     *
     * @param bc a valid ByzCoin instance
     * @param darcID id of a darc allowed to spawn longTermSecret
     * @param signers needed to authenticate longTermSecret spawns
     * @param roster if given, the roster for the DKG, if null, the full roster of bc will be used
     */
    static async spawn(bc: ByzCoinRPC, darcID: InstanceID, signers: [Signer], roster?: Roster):
        Promise<LongTermSecret> {
        if (!roster) {
            roster = bc.getConfig().roster;
        }
        const ocs = new OnChainSecretRPC(bc);
        const lr = await ocs.createLTS(roster, darcID, signers);
        return new LongTermSecret(bc, lr.instanceid, lr.X, roster);
    }

    constructor(bc: ByzCoinRPC, public id: InstanceID, public X: Point, roster?: Roster) {
        super(bc, roster);
    }
}

/**
 * Authorize is used to add the given ByzCoinID into the list of authorized IDs.
 */
export class Authorize extends Message<Authorize> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("Authorize", Authorize);
    }

    readonly byzcoinid: InstanceID;

    constructor(props?: Properties<Authorize>) {
        super(props);
    }
}

/**
 * AuthorizeReply is returned upon successful authorisation.
 */
export class AuthorizeReply extends Message<AuthorizeReply> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("AuthorizeReply", AuthorizeReply);
    }
}

//
/**
 * CreateLTS is used to start a DKG and store the private keys in each node.
 * Prior to using this request, the Calypso roster must be recorded on the
 * ByzCoin blockchain in the instance specified by InstanceID.
 */
export class CreateLTS extends Message<CreateLTS> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("CreateLTS", CreateLTS);
    }

    readonly proof: Proof;

    constructor(props?: Properties<CreateLTS>) {
        super(props);
    }
}

/**
 * CreateLTSReply is returned upon successfully setting up the distributed
 * key.
 */
export class CreateLTSReply extends Message<CreateLTSReply> {

    get X(): Point {
        return PointFactory.fromProto(this.x);
    }

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("CreateLTSReply", CreateLTSReply);
    }

    readonly byzcoinid: InstanceID;
    readonly instanceid: InstanceID;
    readonly x: Buffer;
}

/**
 * DecryptKey is sent by a reader after he successfully stored a 'Read' request
 * in byzcoin Client.
 */
export class DecryptKey extends Message<DecryptKey> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("DecryptKey", DecryptKey);
    }

    readonly read: Proof;
    readonly write: Proof;

    constructor(props?: Properties<DecryptKey>) {
        super(props);
    }
}

/**
 * DecryptKeyReply is returned if the service verified successfully that the
 * decryption request is valid.
 */
export class DecryptKeyReply extends Message<DecryptKeyReply> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("DecryptKeyReply", DecryptKeyReply);
    }
    readonly c: Buffer;
    readonly xhatenc: Buffer;
    readonly x: Buffer;

    async decrypt(priv: Scalar): Promise<Buffer> {
        const X = PointFactory.fromProto(this.x);
        const C = PointFactory.fromProto(this.c);
        /* tslint:disable-next-line: variable-name */
        const XhatEnc = PointFactory.fromProto(this.xhatenc);
        return DecodeKey(X, C, XhatEnc, priv);
    }
}

/**
 * LtsInstanceInfo is the information stored in an LTS instance.
 */
export class LtsInstanceInfo extends Message<LtsInstanceInfo> {

    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("LtsInstanceInfo", LtsInstanceInfo);
    }

    readonly roster: Roster;

    constructor(props?: Properties<LtsInstanceInfo>) {
        super(props);
    }
}

Authorize.register();
AuthorizeReply.register();
CreateLTS.register();
CreateLTSReply.register();
DecryptKey.register();
DecryptKeyReply.register();
LtsInstanceInfo.register();
