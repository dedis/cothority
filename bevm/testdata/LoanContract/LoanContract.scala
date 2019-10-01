import stainless.smartcontracts._
import stainless.annotation._
import stainless.collection._
import stainless.lang.StaticChecks._
import stainless.lang.old
import stainless.lang.ghost
import scala.language.postfixOps

import scala.annotation.meta.field

import LoanContractInvariant._

/************************************************
**  See report for a detail explanation of the
**  contract 
*************************************************/

sealed trait State
case object WaitingForData extends State
case object WaitingForLender extends State
case object WaitingForPayback extends State
case object Finished extends State
case object Default extends State

sealed case class LoanContract (
    val borrower: Address,      // Amount of ether to borrow
    val wantedAmount: Uint256,   // Interest in ether
    val premiumAmount: Uint256,  // The amount of digital token guaranteed
    val tokenAmount: Uint256,    // Name of the digital token
    val tokenName: String,      // Reference to the contract that holds the tokens
    val tokenContractAddress: ERC20Token,
    val daysToLend: Uint256,
    var currentState: State,
    var start: Uint256,
    var lender: Address,

    @ghost
    var visitedStates: List[State]

)  extends Contract {
    require(
        addr != borrower &&
        addr != tokenContractAddress.addr
    )

    override def addr = Address(1)

    def checkTokens(): Unit = {
        require(invariant(this))

        if(currentState == WaitingForData) {
            val balance = tokenContractAddress.balanceOf(addr)
            if(balance >= tokenAmount) {
                ghost {
                    visitedStates = WaitingForLender :: visitedStates
                }
                currentState = WaitingForLender
            }
        }
    } ensuring { _ =>
        invariant(this)
    }

    @payable
    def lend(): Unit = {
        require (invariant(this))

        // Condition to prevent self funding.
        if(Msg.sender != borrower) {
            if(currentState == WaitingForLender && Msg.value >= wantedAmount) {
                lender = Msg.sender
                // Forward the money to the borrower
                borrower.transfer(wantedAmount)
                ghost {
                    visitedStates = WaitingForPayback :: visitedStates
                }

                currentState = WaitingForPayback
                start = now()
            }
        }
    } ensuring { _ =>
        invariant(this)
    }

    @payable
    def payback(): Unit = {
        require (invariant(this))
        dynRequire(address(this).balance >= Msg.value)
        dynRequire(Msg.value >= premiumAmount + wantedAmount)
        dynRequire(Msg.sender == lender)

        if(currentState == WaitingForPayback) {
            // Forward the money to the lender
            lender.transfer(Msg.value)
            // Transfer all the guarantee back to the borrower
            val balance = tokenContractAddress.balanceOf(addr)
            tokenContractAddress.transfer(borrower, balance)
            ghost {
                visitedStates = Finished :: visitedStates
            }

            currentState = Finished
        }
    } ensuring { _ =>
        invariant(this)
    }

    def requestDefault(): Unit = {
        require (invariant(this))

        if(currentState == WaitingForPayback) {
            dynRequire(now() > (start + daysToLend))
            dynRequire(Msg.sender == borrower)

            // Transfer all the guarantee to the lender
            var balance = tokenContractAddress.balanceOf(addr)
            
            tokenContractAddress.transfer(lender, balance)
            ghost {
                visitedStates = Default :: visitedStates
            }
            
            currentState = Default
        }
    } ensuring { _ =>
        invariant(this)
    }
}
