import CredentialsInstance, { Attribute, Credential, CredentialStruct } from "../../personhood/credentials-instance";
import { PopPartyInstance } from "../../personhood/pop-party-instance";
import * as PopPartyProto from "../../personhood/proto";
import RoPaSciInstance, { RoPaSciStruct } from "../../personhood/ro-pa-sci-instance";
import SpawnerInstance, { SpawnerStruct, SPAWNER_COIN } from "../../personhood/spawner-instance";
import CoinInstance, { Coin } from "./coin-instance";
import DarcInstance from "./darc-instance";

const coin = {
    Coin,
    CoinInstance
}

export {
    coin,
    Coin,
    CoinInstance,
    Attribute,
    Credential,
    CredentialStruct,
    CredentialsInstance,
    DarcInstance,
    PopPartyInstance,
    PopPartyProto,
    RoPaSciInstance,
    RoPaSciStruct,
    SpawnerInstance,
    SpawnerStruct,
    SPAWNER_COIN
};
