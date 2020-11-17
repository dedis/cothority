import { Argument } from "../index";
import { IBeautifyArgument } from "./utils";

/**
 * Default representation of arguments
 */
export class DefaultBeautifier {
    static Hex(args: Argument[]): IBeautifyArgument[] {
        const res = Array<IBeautifyArgument>();

        args.forEach((arg) => {
            res.push({name: arg.name, value: arg.value.toString("hex")});
        });

        return res;
    }
}
