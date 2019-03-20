import stainless.smartcontracts._

import stainless.collection._
import stainless.proof._
import stainless.lang._
import stainless.annotation._

object ERC20Specs {
    def transferUpdate(a: Address, to: Address, sender: Address, amount: Uint256, thiss: ERC20Token, oldThiss: ERC20Token) = {
        ((a == to) ==> (thiss.balanceOf(a) == oldThiss.balanceOf(a) + amount)) &&
        ((a == sender) ==> (thiss.balanceOf(a) == oldThiss.balanceOf(a) - amount)) &&
        (a != to && a != sender) ==> (thiss.balanceOf(a) == oldThiss.balanceOf(a))
    }

    def transferSpec(b: Boolean, to: Address, sender: Address, amount: Uint256, thiss: ERC20Token, oldThiss: ERC20Token) = {
        (!b ==> (thiss == oldThiss)) &&
        (b ==> forall((a: Address) => transferUpdate(a, to, sender,amount,thiss,oldThiss))) &&
            (thiss.addr == oldThiss.addr)
    }

    def snapshot(token: ERC20Token): ERC20Token = {
        val ERC20Token(s) = token
        ERC20Token(s)
    }
}