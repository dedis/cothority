import { G1, G2, GT } from "./bn";
/**
 * Compute the pairing between a point in G1 and a point in G2
 * using the Optimal Ate algorithm
 * @param g1 the point in G1
 * @param g2 the point in G2
 * @returns the resulting point in GT
 */
export declare function optimalAte(g1: G1, g2: G2): GT;
