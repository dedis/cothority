import {InstanceID} from "~/lib/cothority/byzcoin/ClientTransaction";

const curve = require("@dedis/kyber-js").curve.newCurve("edwards25519");
const Schnorr = require("@dedis/kyber-js").sign.schnorr;
import {Identity} from "~/lib/cothority/darc/Identity";
import {IdentityEd25519} from "~/lib/cothority/darc/IdentityEd25519";
import {Darc} from "~/lib/cothority/darc/Darc";

/**
 * @extends Identity
 */
export class IdentityDarc extends Identity {
  /**
   * @param {Point}pub
   */
  constructor(public iid: InstanceID) {
    super();
  }

  toString() {
    return this.typeString() + ":" + this.iid.iid.toString('hex');
  }

  typeString() {
    return "darc";
  }

  /**
   * Create an object with all the necessary field needed to be a valid message
   * in the sense of protobufjs. This object can then be used with the "create"
   * method of protobuf
   *
   * @return {Object}
   */
  toObject(): object {
    return {
      darc: {
        id: this.iid.iid,
      }
    };
  }

  /**
   * Creates an IdentityDarc from an object representation.
   * @param {Object} proto
   * @return {IdentityDarc}
   */
  static fromObject(obj: any): IdentityDarc {
    return new IdentityDarc(new InstanceID(Buffer.from(obj.darc.id)));
  }

  /**
   * Creates an identity from a public key
   * @param {Uint8Array} publicKey - the key
   * @return {IdentityEd25519} - the identity
   */
  static fromDarc(d: Darc): IdentityDarc {
    return new IdentityDarc(new InstanceID(d.getBaseId()));
  }
}
