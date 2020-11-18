import { Argument } from "..";

export interface IBeautifierSchema {
    status: 0 | 1;
    type: "spawn" | "invoke" | "delete";
    contract: string;
    args: IBeautifyArgument[];
}

export interface IBeautifyArgument {
    name: string;
    value: string;
    full?: string;
}
