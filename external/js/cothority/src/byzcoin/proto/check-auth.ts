import { Message, Properties } from "protobufjs/light";
import IdentityWrapper from "../../darc/identity-wrapper";
import { registerMessage } from "../../protobuf";
import ClientTransaction from "../client-transaction";
import { InstanceID } from "../instance";

export default class CheckAuthorization extends Message<CheckAuthorization> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.CheckAuthorization", CheckAuthorization, ClientTransaction);
    }

    readonly version: number;
    readonly byzcoinID: InstanceID;
    readonly darcID: InstanceID;
    readonly identities: IdentityWrapper[];

    constructor(props?: Properties<CheckAuthorization>) {
        super(props);

        /* Protobuf aliases */

        Object.defineProperty(this, "byzcoinid", {
            get(): InstanceID {
                return this.byzcoinID;
            },
            set(value: InstanceID) {
                this.byzcoinID = value;
            },
        });

        Object.defineProperty(this, "darcid", {
            get(): InstanceID {
                return this.darcID;
            },
            set(value: InstanceID) {
                this.darcID = value;
            },
        });
    }
}
export  class CheckAuthorizationResponse extends Message<CheckAuthorizationResponse> {
    /**
     * @see README#Message classes
     */
    static register() {
        registerMessage("byzcoin.CheckAuthorizationResponse", CheckAuthorizationResponse, ClientTransaction);
    }

    readonly actions: string[];

    constructor(props?: Properties<CheckAuthorizationResponse>) {
        super(props);
    }
}

CheckAuthorization.register();
CheckAuthorizationResponse.register();
