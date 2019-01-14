import Constants from '../src/constants'

describe('Kyber constants', () => {
    it('should provide the 0 constant using big number', () => {
        expect(Constants.zeroBN).toBeDefined();
    });
});
