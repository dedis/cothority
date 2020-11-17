import { Darc } from "../../darc";
import { Roster } from "../../network";
import { Argument } from "../index";
import { ChainConfig } from "../index";
import { IBeautifyArgument } from "./utils";
// tslint:disable-next-line
const varint = require("varint");

/**
 * Arrange arguments for a config contract, ie. provide a meaningful
 * representation of its arguments.
 */
export class ConfigBeautifier {
    static Spawn(args: Argument[]): IBeautifyArgument[] {
        const res = Array<IBeautifyArgument>();

        args.forEach((arg) => {
            switch (arg.name) {
                case "darc":
                    const darc = Darc.decode(arg.value);
                    res.push({name: "darc", value: darc.description.toString(), full: darc.toString()});
                    break;
                case "block_interval":
                    res.push({name: "block_interval", value: `${varint.decode(arg.value, 0) / 1e6} ms`});
                    break;
                case "max_block_size":
                    res.push({name: "max_block_size", value: `${varint.decode(arg.value, 0)} bytes`});
                    break;
                case "roster":
                    const r = Roster.decode(arg.value);
                    res.push({name: "roster", value: r.id.toString("hex"), full: r.toTOML()});
                    break;
                case "trie_nonce":
                    res.push({name: "trie_nonce", value: `${varint.decode(arg.value, 0)}`});
                    break;
                case "darc_contracts":
                    res.push({name: "darc_contracts", value: arg.value.toString("hex")});
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
                case "config":
                    // Calypso tests won't pass if we use ChainConfig.
                    // const config = ChainConfig.decode(arg.value);
                    // res.push({name: "darc", value: "chain config", full: config.toString()});
                    res.push({name: "darc", value: "chain config"});
                    break;
                default:
                    res.push({name: arg.name, value: "unspecified", full: arg.value.toString("hex")});
                    break;
            }
        });

        return res;
    }
}
