import stainless.smartcontracts._
import stainless.lang._
import stainless.collection._
import stainless.annotation._

object LoanContractInvariant {
  @ghost
  def invariant(
    contract: LoanContract
  ) = {
    tokenInvariant(address(contract), contract.currentState, contract.tokenAmount, contract.tokenContractAddress) &&
    stateInvariant(contract.currentState, contract.visitedStates)
  }

  def tokenInvariant(
    loanContractAddress: Address,
    contractState: State,
    tokenAmount: Uint256,
    tokenContractAddress: ERC20Token
  ): Boolean = {
    (contractState == WaitingForLender ==> (tokenContractAddress.balanceOf(loanContractAddress) >= tokenAmount)) &&
    (contractState == WaitingForPayback ==> (tokenContractAddress.balanceOf(loanContractAddress) >= tokenAmount))
  }

  def isPrefix[T](l1: List[T], l2: List[T]): Boolean = (l1,l2) match {
    case (Nil(), _) => true
    case (Cons(x, xs), Cons(y, ys)) => x == y && isPrefix(xs, ys)
    case _ => false
  }

  @ghost
  def stateInvariant(
    currentState: State,
    visitedStates: List[State]
  ) = {
    val expected1: List[State] = List(WaitingForData, WaitingForLender, WaitingForPayback, Finished)
    val expected2: List[State] = List(WaitingForData, WaitingForLender, WaitingForPayback, Default)
    val rStates = visitedStates.reverse

    visitedStates.contains(WaitingForData) && 
    visitedStates.head == currentState && (
      isPrefix(rStates, expected1) ||
      isPrefix(rStates, expected2)
    )
  }
}

