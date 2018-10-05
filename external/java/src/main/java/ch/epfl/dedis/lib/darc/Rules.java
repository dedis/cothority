package ch.epfl.dedis.lib.darc;

import ch.epfl.dedis.lib.exception.CothorityAlreadyExistsException;
import ch.epfl.dedis.lib.exception.CothorityNotFoundException;
import ch.epfl.dedis.lib.proto.DarcProto;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;

/**
 * Rules is a list of action-expression associations.
 */
public class Rules {
    public final static String OR = " | ";
    public final static String AND = " & ";

    private List<Rule> list;

    /**
     * Default constructor that creates an empty rule list.
     */
    public Rules() {
        this.list = new ArrayList<>();
    }

    /**
     * This is the copy constructor.
     * @param other
     */
    public Rules(Rules other) {
        List<Rule> newList = new ArrayList<>(other.list.size());
        newList.addAll(other.list);
        this.list = newList;
    }

    /**
     * Constructor for the protobuf representation.
     * @param rules
     */
    public Rules(DarcProto.Rules rules) {
        this.list = new ArrayList<>();
        for (DarcProto.Rule protoRule : rules.getListList()) {
            Rule r = new Rule(protoRule.getAction(), protoRule.getExpr().toByteArray());
            this.list.add(r);
        }
    }

    /**
     * Adds a rule. CothorityAlreadyExistsException is thrown if the action that the function is trying to add already
     * exists.
     * @param a is the action
     * @param expr is the expression
     * @throws CothorityAlreadyExistsException
     */
    public void addRule(String a, byte[] expr) throws CothorityAlreadyExistsException {
        if (exists(a) != -1) {
            throw new CothorityAlreadyExistsException("rule already exists");
        }
        list.add(new Rule(a, expr));
    }

    /**
     * Updates a rule. CothorityNotFoundException is thrown if the action that we are trying to update does not exist.
     * @param a is the action
     * @param expr is the expression
     * @throws CothorityNotFoundException
     */
    public void updateRule(String a, byte[] expr) throws CothorityNotFoundException {
        int i = exists(a);
        if (i == -1) {
            throw new CothorityNotFoundException("cannot update a non-existing rule");
        }
        this.list.set(i, new Rule(a, expr));
    }

    /**
     * Gets a rule, if it does not exist then null is returned.
     * @param a is the action
     * @return
     */
    public Rule get(String a) {
        for (Rule rule : this.list) {
            if (rule.getAction().equals(a)) {
                return rule;
            }
        }
        return null;
    }

    /**
     * Gets all rules as a List.
     * @return
     */
    public List<Rule> getAllRules() {
        return this.list;
    }

    /**
     * Gets all the actions as a List.
     * @return
     */
    public List<String> getAllActions() {
        return this.list.stream().map(Rule::getAction).collect(Collectors.toList());
    }

    /**
     * Removes the action if it exists.
     * @param a is the action
     * @return the removed Rule if it exists, otherwise null.
     */
    public Rule remove(String a) {
        int i = exists(a);
        if (i == -1) {
            return null;
        }
        return this.list.remove(i);
    }

    /**
     * Checks whether a rule exists.
     * @param a is the action
     * @return
     */
    public boolean contains(String a) {
        return exists(a) != -1;
    }

    /**
     * Converts the rule to its protobuf representation.
     * @return the protobuf representation
     */
    public DarcProto.Rules toProto() {
        DarcProto.Rules.Builder b = DarcProto.Rules.newBuilder();
        for (Rule rule : this.list) {
            b.addList(rule.toProto());
        }
        return b.build();
    }

    private int exists(String a) {
        for (int i = 0; i < list.size(); i++) {
            if (list.get(i).getAction().equals(a)) {
                return i;
            }
        }
        return -1;
    }
}
