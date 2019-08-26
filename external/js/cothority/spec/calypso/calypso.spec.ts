import { curve } from "@dedis/kyber";
import Keccak from "keccak";
import Long from "long";
import { ByzCoinRPC } from "../../src/byzcoin";
import { CalypsoReadInstance, CalypsoWriteInstance, Write } from "../../src/calypso/calypso-instance";
import { LongTermSecret, OnChainSecretRPC } from "../../src/calypso/calypso-rpc";
import { SignerEd25519 } from "../../src/darc";
import Darc from "../../src/darc/darc";
import { Rule } from "../../src/darc/rules";
import SpawnerInstance from "../../src/personhood/spawner-instance";
import { BLOCK_INTERVAL, ROSTER, SIGNER, startConodes } from "../support/conondes";

const curve25519 = curve.newCurve("edwards25519");

describe("Keccak Tests", () => {
    it("should return squeezed data", () => {
        /* Created from go with:
            k := keccak.New([]byte("keccak message"))
            for i := 0; i < 10; i++{
                    out := make([]byte, i)
                    k.Read(out)
                    fmt.Printf("'%x',\n", out)
            }
         */
        const res = [
            "",
            "9f",
            "66ac",
            "9d3b8d",
            "024d6440",
            "49369f76b2",
            "42161ce754b7",
            "56a9b322a98b6d",
            "a7d38c6ef3d29b29",
            "5eb668c1eb315106e4",
        ];
        const k = new Keccak("shake256");
        k.update(Buffer.from("keccak message"));
        for (let i = 0; i < 10; i++) {
            expect(k.squeeze(i)).toEqual(Buffer.from(res[i], "hex"));
        }
    });
});

describe("Offline Calypso Tests", () => {
    it("return the same as in go", async () => {
        /* Go-file:
        ltsid := byzcoin.NewInstanceID([]byte("LTS Instance ID"))
        writeDarc := darc.ID(byzcoin.NewInstanceID([]byte("Write Darc ID")).Slice())
        X := cothority.Suite.Point().Embed([]byte("Aggregate public key"), keccak.New([]byte("public")))
        key := []byte("Very Secret Symmetric Key")
        w := calypso.NewWrite(cothority.Suite, ltsid, writeDarc, X, key, keccak.New(ltsid.Slice()))
        log.Printf("ltsID: %x", ltsid[:])
        log.Printf("writeDarc: %x", writeDarc)
        Xbuf, _ := X.MarshalBinary()
        log.Printf("X: %x", Xbuf)
        log.Printf("key: %x", key)
        log.Printf("w: %+v", w)
         */
        const ltsID = Buffer.from("4c545320496e7374616e63652049440000000000000000000000000000000000", "hex");
        const writeDarc = Buffer.from("5772697465204461726320494400000000000000000000000000000000000000", "hex");
        const X = curve25519.point();
        X.unmarshalBinary(Buffer.from("14416767726567617465207075626c6963206b65796445b49ac5ec4c9161e706", "hex"));
        const key = Buffer.from("56657279205365637265742053796d6d6574726963204b6579", "hex");

        const k = new Keccak("shake256");
        k.update(ltsID);
        const wr = await Write.createWrite(ltsID, writeDarc, X, key, (l) => k.squeeze(l));

        const U = curve25519.point();
        U.unmarshalBinary(Buffer.from("946de817c1bd2465559ba9c5c0def6feeb6a3b842e9b6ff86d34b638a41f11ed", "hex"));
        // tslint:disable-next-line
        const Ubar = curve25519.point();
        Ubar.unmarshalBinary(Buffer.from("c47944aacc329efcff490e5b4cf79c4706c6a5eaa1341b0afa54bc9dcaf581f0", "hex"));
        const E = curve25519.scalar();
        E.unmarshalBinary(Buffer.from("c4c4d3aa5b2a6dea627c3c843a1d407d748b222af45472b7015de160618fcf09", "hex"));
        const F = curve25519.scalar();
        F.unmarshalBinary(Buffer.from("f940db2931d055e08c330cfffeeefa30ccf0638b2cb0d779725b5ccb2781510a", "hex"));
        const C = curve25519.point();
        C.unmarshalBinary(Buffer.from("767057c87242f52c06e5e7c44d67fa92a04f20dd8dd373b5f818923648290f4b", "hex"));

        expect(wr.u.equals(U.marshalBinary())).toBeTruthy();
        expect(wr.ubar.equals(Ubar.marshalBinary())).toBeTruthy();
        expect(wr.e.equals(E.marshalBinary())).toBeTruthy();
        expect(wr.f.equals(F.marshalBinary())).toBeTruthy();
        expect(wr.c.equals(C.marshalBinary())).toBeTruthy();
        expect(wr.ltsid.equals(ltsID)).toBeTruthy();
    });
});

describe("Online Calypso Tests", async () => {
    let bc: ByzCoinRPC;
    let darc: Darc;
    let lts: LongTermSecret;
    let spawner: SpawnerInstance;

    beforeAll(async () => {
        await startConodes();
        const roster = ROSTER.slice(0, 4);
        darc = ByzCoinRPC.makeGenesisDarc([SIGNER], roster);
        ["spawn:longTermSecret", "spawn:credential", "invoke:credential.update",
            "spawn:calypsoWrite", "spawn:calypsoRead", "spawn:spawner"].forEach((rule) =>
            darc.addIdentity(rule, SIGNER, Rule.OR));

        bc = await ByzCoinRPC.newByzCoinRPC(roster, darc, BLOCK_INTERVAL);
        const ocs = new OnChainSecretRPC(bc);
        await ocs.authorizeRoster();
        lts = await LongTermSecret.spawn(bc, darc.id, [SIGNER], roster);
        // A second authorisation-request should return an error from the conode
        // which should be handled by the `spawn` method.
        await LongTermSecret.spawn(bc, darc.id, [SIGNER], roster);
        const costs = {
            costCRead: Long.fromNumber(100),
            costCWrite: Long.fromNumber(1000),
            costCoin: Long.fromNumber(100),
            costCredential: Long.fromNumber(1000),
            costDarc: Long.fromNumber(100),
            costParty: Long.fromNumber(1000),
            costRoPaSci: Long.fromNumber(10),
        };
        spawner = await SpawnerInstance.spawn({
            bc,
            beneficiary: null,
            costs,
            darcID: darc.id,
            signers: [SIGNER],
        });
    });

    it("should be able to create an LTS", async () => {
        const key = Buffer.from("Very Secret Key");

        const wr = await Write.createWrite(lts.id, darc.getBaseID(), lts.X, key);
        const wrInst = await CalypsoWriteInstance.spawn(bc, darc.getBaseID(), wr, [SIGNER]);

        const signer = SignerEd25519.random();
        const readInst = await CalypsoReadInstance.spawn(bc, wrInst.id, signer.public, [SIGNER]);
        const decrypt = await lts.reencryptKey(await bc.getProof(wrInst.id),
            await bc.getProof(readInst.id));
        const newKey = await decrypt.decrypt(signer.secret);
        expect(newKey).toEqual(key);
    });

    it("should create an LTS and a write using the spawner", async () => {
        const key = Buffer.from("Very Secret Key");

        const write = await Write.createWrite(lts.id, darc.id, lts.X, key);
        const wrInst = await CalypsoWriteInstance.spawn(bc, darc.id, write, [SIGNER]);

        const signer = SignerEd25519.random();
        const readInst = await wrInst.spawnRead(signer.public, [SIGNER]);
        const newKey = await readInst.decrypt(lts, signer.secret);
        expect(newKey).toEqual(key);
    });
});
