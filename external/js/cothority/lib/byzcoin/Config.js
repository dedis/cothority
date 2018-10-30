const root = require("../protobuf/index.js").root;
const identity = require("../identity");

/**
 * Config is the genesis configuration of a ByzCoin instance. It can be stored only once in ByzCoin
 * and defines the basic running parameters of ByzCoin.
 */
class Config {
  /**
   * Creates a config from knwon informations
   * @param {number} blockInterval
   * @param {Roster} roster that hosts the ByzCoin ledger
   */
  constructor(blockInterval, roster) {
    this._blockInterval = blockInterval;
    this._roster = roster;
  }

  /**
   * @return {number} - the blockinterval used
   */
  get blockInterval() {
    return this._blockInterval;
  }

  /**
   * @return {Roster} - the roster of the byzcoin
   */
  get roster() {
    return this._roster;
  }

  /**
   * Creates a Config from a byte array
   * @param {Uint8Array} buf
   * @return {Config}
   */
  static fromByteBuffer(buf) {
    if (!(buf instanceof Uint8Array)) {
      throw "buf must be of type Uint8Array in Config";
    }
    const configModel = root.lookup("ChainConfig");
    let config = configModel.decode(buf);

    return new Config(config.blockinterval, identity.Roster.fromProtobuf(config.roster, false));
  }

  /**
   * Check if two Configs are equal
   * @param {Object} config
   * @return {boolean} - true if the config are equals (e.i if they have
   * the same blockinterval)
   */
  equals(config) {
    if (config === undefined) return false;
    if (!(config instanceof Config)) return false;
    return this._blockInterval === config.blockInterval;
  }
}

module.exports = Config;
