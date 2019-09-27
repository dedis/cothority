import stainless.smartcontracts._

import stainless.collection._
import stainless.proof._
import stainless.lang._
import stainless.annotation._

import ERC20Specs._

case class ERC20Token(var s: BigInt) extends ContractInterface {
    @library
    def transfer(to: Address, amount: Uint256): Boolean = {
        require(amount >= Uint256.ZERO)
        val oldd = snapshot(this)
        s = s + 1

        val b = choose((b: Boolean) => transferSpec(b, to, Msg.sender, amount, this, oldd))
        b
    } ensuring(res => transferSpec(res, to, Msg.sender, amount, this, old(this)))

    @library
    def balanceOf(from: Address): Uint256 = {
        choose((b: Uint256) => b >= Uint256.ZERO)
    } ensuring {
        res => old(this).addr == this.addr
    }
}

