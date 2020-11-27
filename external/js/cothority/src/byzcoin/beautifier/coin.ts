import Long from "long";
import { Argument } from "..";
import { Darc } from "../../darc";
import { IBeautifyArgument } from "./utils";

/**
 * Arrange arguments for a Coin contract, ie. provide a meaningful
 * representation of its arguments.
 */
export class CoinBeautifier {
    static Spawn(args: Argument[]): IBeautifyArgument[] {
        const res = Array<IBeautifyArgument>();

        args.forEach((arg) => {
            switch (arg.name) {
                case "public":
                    res.push({name: arg.name, value: arg.value.toString("hex")});
                    break;
                case "coinID":
                    res.push({name: arg.name, value: arg.value.toString("hex")});
                    break;
                case "darcID":
                    try {
                        const darc = Darc.decode(arg.value);
                        res.push({name: "darc", value: darc.description.toString(), full: darc.toString()});
                    } catch (e) {
                        // case for the block d487b398d47cb2893baa2bf17dd7b00511840b46f58563d44d67bffca43d5de0
                        res.push({full: `error: ${e} - darc: ${arg.value.toString()}`,
                        name: "darc", value: "failed to decode"});
                    }
                    break;
                case "type":
                    res.push({name: arg.name, value: arg.value.toString("hex")});
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
                case "coins":
                    const coins = Long.fromBytesLE(Array.from(arg.value));

                    let label = "coin";
                    if (coins.compare(1) > 0) {
                        label = "coins";
                    }

                    res.push({name: "coins", value: `${coins} ${label}`});
                    break;
                case "destination":
                    res.push({name: "destination", value: arg.value.toString("hex")});
                    break;
                default:
                    res.push({name: arg.name, value: "unspecified", full: arg.value.toString("hex")});
                    break;
            }
        });

        return res;
    }
}
