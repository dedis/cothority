import { Argument } from "..";
import { Credential } from "../../personhood/credentials-instance";
import { IBeautifyArgument } from "./utils";

/**
 * Arrange arguments for a Credential contract, ie. provide a meaningful
 * representation of its arguments.
 */
export class CredentialBeautifier {
    static Spawn(args: Argument[]): IBeautifyArgument[] {
        const res = Array<IBeautifyArgument>();

        args.forEach((arg) => {
            switch (arg.name) {
                case "darcIDBuf":
                    res.push({name: arg.name, value: arg.value.toString("hex")});
                    break;
                case "credentialID":
                    res.push({name: arg.name, value: arg.value.toString("hex")});
                    break;
                case "credential":
                    const cred = Credential.decode(arg.value);
                    res.push({name: arg.name, value: "credential", full: cred.toString()});
                    break;
                default:
                    res.push({name: arg.name, value: "unspecified", full: arg.value.toString("hex")});
                    break;
            }
        });

        return res;
    }
    static Invoke(args: Argument[]): IBeautifyArgument[] {
        const res = Array<IBeautifyArgument>();

        args.forEach((arg) => {
            switch (arg.name) {
                case "credential":
                    const cred = Credential.decode(arg.value);
                    res.push({name: "credential", value: "credential", full: cred.toString()});
                    break;
                case "signatures":
                    res.push({name: "signatures", value: arg.value.toString("hex")});
                    break;
                case "public":
                    res.push({name: "public", value: arg.value.toString("hex")});
                    break;
                default:
                    res.push({name: arg.name, value: "unspecified", full: arg.value.toString("hex")});
                    break;
            }
        });

        return res;
    }
}
