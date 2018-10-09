package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.proto.DarcProto;
import com.google.protobuf.ByteString;

/**
 * Rule is a pair of action and expression.
 */
public final class Rule {
    private String action;
    private byte[] expr;

    /**
     * Constructor for creating a rule.
     * @param action the action
     * @param expr the expression
     */
    public Rule(String action, byte[] expr) {
        this.action = action;
        this.expr = expr;
    }

    /**
     * Getter for action.
     * @return the action
     */
    public String getAction() {
        return action;
    }

    /**
     * Getter for expression.
     * @return the expression
     */
    public byte[] getExpr() {
        return expr;
    }

    /**
     * Converts the object to its equivalent protobuf representation.
     * @return the protobuf representation
     */
    public DarcProto.Rule toProto() {
        DarcProto.Rule.Builder b = DarcProto.Rule.newBuilder();
        b.setAction(this.action);
        b.setExpr(ByteString.copyFrom(this.expr));
        return b.build();
    }
}
