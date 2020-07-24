package bls

import (
	"errors"
	"fmt"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/util/random"
	"math/rand"
)

// =========================================================
// shamir secret share

// 密钥保存为scalar
// 公钥保存为G2.point
// 签名保存为G1.point

// 多项式，每一项都是一个bls密钥，其中第一项是公共密钥
type Polynome []kyber.Scalar

// test case: f(x) = 2 + x + 3x^2 + x^3
// 传入一个公共私钥，返回一个指定阶数的多项式(d+1)
// 第1项为传入值x，其余各项均为随机数
func Master(x kyber.Scalar, d int) Polynome {
	p := make([]kyber.Scalar, d+1, d+1)
	p[0] = x

	for i := 0; i < d; i++ {
		tmp := rand.New(rand.NewSource(int64(i)))
		cipher1 := random.New(tmp)
		p[i+1], _ = bls.NewKeyPair(bn256_suite, cipher1)
	}
	return p
}

// 计算p(idx)的值
// todo idx的编码问题，暂定int64
func (p Polynome) GetValue(idx int64) (kyber.Scalar, error) {
	return p.getValue(bn256_suite, idx)
}

func (p Polynome) getValue(suite *bn256.Suite, idx int64) (kyber.Scalar, error) {
	if p == nil {
		return nil, errors.New("多项式不能为空")
	}

	if suite == nil {
		suite = bn256.NewSuite()
	}

	res := suite.G2().Scalar().One().Set(p[0])
	base := suite.G2().Scalar().One()
	idxScalar := suite.G2().Scalar().One().SetInt64(idx)
	for i := 1; i < len(p); i++ {
		base.Mul(base, idxScalar) // x^i = x^i-1 * x
		tmp := suite.G2().Scalar().One()
		tmp = tmp.Mul(p[i], base) // a_i * x_i ^ i
		res = res.Add(res, tmp)   // res += a_i * x_i ^ i
	}

	return res, nil
}

// 利用拉格朗日插值法，从n个子签名中还原出一个唯一的签名
// ids和sigs一定一一对应，否则计算出来的结果会有问题
// ids中的每一个值都是唯一的，这里的取值可以是节点的peerID，待定
// 但签名个数小于阈值threshold value时，会返回一个空的签名和一个错误
func SignatureRecovery(threshold int, sigs [][]byte, ids []int64) ([]byte, error) {

	if len(ids) != len(sigs) {
		return nil, errors.New("签名和id个数不相等")
	}

	if len(ids) == 0 || len(sigs) == 0 {
		return nil, errors.New("签名或者id不能为空")
	}
	if len(sigs) <= threshold {
		return nil, errors.New("签名个数少于阈值")
	}

	if len(sigs) == 1 {
		return sigs[0], nil
	}

	return recovery(bn256_suite, sigs, ids)
}

// calculate f(0)
// f(x) = \sum_i{y_i \times \frac{a}{b_i}} where a = \prod_ix_i and b_i = x_i \times \prod_{i!=j}{x_j - x_i}
// 签名结果在G1上运算
func recovery(suite *bn256.Suite, ys [][]byte, xs []int64) ([]byte, error) {
	if suite == nil {
		suite = bn256.NewSuite()
	}

	lens := len(ys)
	res := suite.G1().Point()

	aScalar := suite.G1().Scalar()
	aScalar.One() // a := 1

	// calulate a
	for _, x := range xs {
		tmp := suite.G1().Scalar().SetInt64(int64(x))
		aScalar.Mul(aScalar, tmp) // a *= x
	}

	for i, y := range ys {
		tmpRes := suite.G1().Point()
		if err := tmpRes.UnmarshalBinary(y); err != nil {
			return nil, errors.New(fmt.Sprintf("第%v个签名还原错误，具体原因: %v", i+1, err.Error()))
		} // tmp = y

		// calculate b_i
		// 一定设置为 xs[i]
		b_i := suite.G1().Scalar().SetInt64(int64(xs[i]))
		for j := 0; j < lens; j++ {
			if i == j {
				continue
			}
			if xs[j] == xs[i] {
				return nil, errors.New("存在id相等的情况")
			}
			b_i_tmp := suite.G1().Scalar().SetInt64(int64(xs[j] - xs[i]))
			b_i.Mul(b_i, b_i_tmp) // b_i *= int64((xs[j] - xs[i]))
		}

		b_i.Div(aScalar, b_i)   // b_i` = a / b_i
		tmpRes.Mul(b_i, tmpRes) // tmpRes = (a / b_i) * y
		res.Add(res, tmpRes)    //res += tmpRes / b_i
	}

	if data, err := res.MarshalBinary(); err != nil {
		return nil, errors.New(fmt.Sprintf("签名序列化错误，具体原因：%v", err.Error()))
	} else {
		return data, nil
	}
}
