import { Argument } from "..";
import { Darc as d } from "../../darc";
import { Roster } from "../../network";
import { IBeautifyArgument } from "./utils";

/**
 * Arrange arguments for a Darc contract, ie. provide a meaningful
 * representation of its arguments.
 */
export class DarcBeautifier {
    static Spawn(args: Argument[]): IBeautifyArgument[] {
        const res = Array<IBeautifyArgument>();

        args.forEach((arg) => {
            switch (arg.name) {
                case "darc":
                    const darc = d.decode(arg.value);
                    res.push({name: "darc", value: darc.description.toString(), full: darc.toString()});
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
                case "darc":
                    const darc = d.decode(arg.value);
                    res.push({name: "darc", value: darc.description.toString(), full: darc.toString()});
                    break;
                default:
                    res.push({name: arg.name, value: "unspecified", full: arg.value.toString("hex")});
                    break;
            }
        });

        return res;
    }
}
