package collection

import "testing"
import csha256 "crypto/sha256"
import "encoding/hex"

func TestSha256(test *testing.T) {
	ctx := testctx("[sha256.go]", test)

	check := func(name string, hexref string, item interface{}, items ...interface{}) {
		reference, _ := hex.DecodeString(hexref)
		response := sha256(item, items...)

		for i := 0; i < csha256.Size; i++ {
			if response[i] != reference[i] {
				test.Error("[sha256.go]", name, "Hash mismatch.")
				return
			}
		}
	}

	check("[bool]", "b413f47d13ee2fe6c845b2ee141af81de858df4ec549a58b7970bb96645bc8d2", true)
	check("[bool]", "96a296d224f285c67bee93c30f8a309157f0daa35dc5b87e410b78630a09cfc7", false)
	check("[int8]", "9e6282e4f25e370ce617e21d6fe265e88b9e7b8682cf00059b9d128d9381f09d", int8(8))
	check("[int8]", "1649d7d9c3337d6e64913d6e659d8d6f021df01c41343a1237dbbecb6c589b7c", int8(-4))
	check("[int16]", "94a9964053c051db20446323bb1c28df4a85ee6200201ae8e9c964290ea0c22c", int16(3072))
	check("[int16]", "b43539de6ef0070b1903d5c74ed1d00e8a416ab60816a31af2206cc94231ae40", int16(-7048))
	check("[int32]", "4970cf9d460a0defb336c8a21ccfcf65f2197356737fe1a7ca117d028f688f13", int32(1048576))
	check("[int32]", "fa6a783186b8e9a741fe174ee59342d1059fdad8e886c41f800d03e5795dbb9e", int32(-7488199))
	check("[int64]", "a2bb951c16ca26dd6e7710953ea318e1fdefff43bd421d26bf7cb2ae14c8a475", int64(274638295294758))
	check("[int64]", "31cbe8fe5568a8219b431c4be58a6fb8a7561af7dc0ff1186ac9119dbcb22c3a", int64(-39857934573614))
	check("[uint8]", "c8a0d4bf3d723b5c59fdfc06d1b33221ae4afb0efe90801cfaa577bd9298cfd1", uint8(44))
	check("[uint16]", "a83c2e8c92b78a43798cfcd6ee8ce84dcc5f2c680966bb2eae017f796adb4e9c", uint16(7791))
	check("[uint32]", "85f46bd1ba19d1014b1179edd451ece95296e4a8c765ba8bba86c16893906398", uint32(563920748))
	check("[uint64]", "673ded6fe028eb8ffa1ab2854c1ac423a29d5bbb02077491463d5633d9288236", uint64(37563738576283756))
	check("[boolslice]", "2007fa88c4ca993381a3e2ea29577c6cd14082e5bb05fc491a775350a64dc927", []bool{false, true, true, true, false, true, true, false, false, false})
	check("[int8slice]", "d915b5e00af86f8781913c9f3b10b59b92a8cf97a3db961b1795acecf4ba28b7", []int8{0, 1, -2, 3, -4, 5, -6, 7, -8, 9, -10})
	check("[int16slice]", "6ee520749d353b8a7b58df89d2ab3c95158e938c0f2c53a5d20adcc8e7c30ab4", []int16{654, -122, 999, -1024})
	check("[int32slice]", "ae4ba3515e058bd62177726cf8f714f531b0f3ad50567dd4fe2d270e3bebbcae", []int32{8375834, -7674835, 0, -1, 2})
	check("[int64slice]", "650ad1c33bfcc7c9bff326ec090b24aabe220562416cc6ad33d5e28d355cb63e", []int64{6738459345, -20985792357, 52739847239572, 0, -1, 2})
	check("[uint8slice]", "06fe6f337daad99b5cb692e82c2720c98f9f906e5c121820a958d9b3c4568221", []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})
	check("[uint16slice]", "979749dac2ce52b939892d587da1c457b28c5974e24018fe185f1fd0674ea97e", []uint16{7412, 14321, 6444, 1000})
	check("[uint32slice]", "da6999d5edbd37d36f7bfdf749ee5adee389275696b3c711d6255ba63b4578e7", []uint32{84729248, 758478753, 7837584, 9991873})
	check("[uint64slice]", "9af6adcb4ff5d82ce2f03d2783f012fd1a22540f52ec7ce12e1caf8ac3e883d6", []uint64{348574985374, 983792837414, 274827365234})
	check("[string]", "6d3b3242ee92087ac4fcdfdd4b9c3afa5e5263b1444e393ce2d0ece2bced68a4", "Hello World!")
	check("[boolsliceslice]", "20070e5626cd2955dd706886333bf098b5cb2438118ae2875e756ea6056a105c", [][]bool{{false, true, false, false}, {true, true}, {true, false, true, false}, {}})
	check("[uint16sliceslice]", "5243de6930574ccd04aa10e0fba32931da43d793c25b9be27db0cd246720f7da", [][]uint16{{0, 1}, {2, 3, 4}, {5, 6, 7, 8, 9}, {10}, {}, {11, 12, 13, 14}, {}, {15, 16}})
	check("[stringslicesliceslice]", "6645aa75ad111853acfcdbc3d00521f056eb889bcc6bb5a8fa6599b6d2535f4f", [][][]string{{{"hello", "world"}, {"this", "is"}}, {{"a", "test"}, {"to", "test"}}, {{"if"}, {"slices", "work", "correctly"}}, {}})
	check("[variadic]", "c4746029b1ace69bea143f17e777bc749f0a6139842321de4f79fa938f1ad799", false, int8(4), int16(1024), int32(29854536), int64(27562875624), uint8(9), uint16(8785), uint32(973583475), uint64(9854398635453), "Hello variadic")

	ctx.should_panic("[int]", func() {
		sha256(33)
	})

	ctx.should_panic("[float]", func() {
		sha256(33.4)
	})

	ctx.should_panic("[array]", func() {
		sha256([3]int32{1, 2, 3})
	})

	ctx.should_panic("[struct]", func() {
		type dummy struct {
			i int
		}

		sha256(dummy{3})
	})

	ctx.should_panic("[pointer]", func() {
		dummy := uint32(4)
		sha256(&dummy)
	})
}
