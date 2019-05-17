import CredentialsInstance, { Attribute, Credential, CredentialStruct } from "../../personhood/credentials-instance";
import { PopPartyInstance } from "../../personhood/pop-party-instance";
import * as PopPartyProto from "../../personhood/proto";
import RoPaSciInstance, { RoPaSciStruct } from "../../personhood/ro-pa-sci-instance";
import SpawnerInstance, { SpawnerStruct } from "../../personhood/spawner-instance";
import CoinInstance, { Coin } from "./coin-instance";
import DarcInstance from "./darc-instance";

const coin = {
    Coin,
    CoinInstance,
};

const credentials = {
    Attribute,
    Credential,
    CredentialStruct,
    CredentialsInstance,
};

const darc = {
    DarcInstance,
};

const pop = {
    PopPartyInstance,
    ...PopPartyProto,
};

const game = {
    RoPaSciInstance,
    RoPaSciStruct,
};

const spawner = {
    SpawnerInstance,
    SpawnerStruct,
};

export {
    coin,
    credentials,
    darc,
    pop,
    game,
    spawner,
};
