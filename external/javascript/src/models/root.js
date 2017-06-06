import protobuf from "protobufjs"
import skeleton from './skeleton'

const {Root} = protobuf;

/**
 * As we need to create a bundle, we cannot use the *.proto files and the a script will wrap
 * them in a skeleton file that contains the JSON representation that can be used in the js code
 */
export default Root.fromJSON(JSON.parse(skeleton));