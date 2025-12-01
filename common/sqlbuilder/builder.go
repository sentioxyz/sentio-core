package builder

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"sentioxyz/sentio-core/common/anyutil"
	"sentioxyz/sentio-core/common/log"
	protoscommon "sentioxyz/sentio-core/service/common/protos"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

var (
	defaultPrefix = "{"
	defaultSuffix = "}"

	parseBigInt = func(v *protoscommon.BigInteger) big.Int {
		var bigint big.Int
		bigint.SetBytes(v.GetData())
		if v.GetNegative() {
			bigint.Neg(&bigint)
		}
		return bigint
	}
	parseBigDecimal = func(v *protoscommon.BigDecimal) decimal.Decimal {
		bigint := parseBigInt(v.GetValue())
		return decimal.NewFromBigInt(&bigint, v.GetExp())
	}
)

type formatOptID int

const (
	parameterIdentity formatOptID = iota
	richStruct
)

type FormatOption struct {
	OptID           formatOptID
	parameterPrefix *string
	parameterSuffix *string

	parameterPrefixList []string
	parameterSuffixList []string

	richStructParameter *protoscommon.RichStruct
	parsedParameter     map[string]any
}

func (f *FormatOption) Merge(opts ...FormatOption) {
	for _, opt := range opts {
		switch opt.OptID {
		case parameterIdentity:
			f.parameterPrefixList = append(f.parameterPrefixList, *opt.parameterPrefix)
			f.parameterSuffixList = append(f.parameterSuffixList, *opt.parameterSuffix)
		case richStruct:
			f.richStructParameter = opt.richStructParameter
			f.parsedParameter = opt.parsedParameter
		}
	}
}

func WithParameterIdentity(prefix, suffix string) FormatOption {
	return FormatOption{
		OptID:           parameterIdentity,
		parameterPrefix: lo.ToPtr(prefix),
		parameterSuffix: lo.ToPtr(suffix),
	}
}

func WithRichStructParameter(richStructParameter *protoscommon.RichStruct) FormatOption {
	m := make(map[string]any)

	for k, v := range richStructParameter.GetFields() {
		switch v.Value.(type) {
		case *protoscommon.RichValue_StringValue:
			m[k] = v.GetStringValue()
		case *protoscommon.RichValue_NullValue_:
			m[k] = nil
		case *protoscommon.RichValue_IntValue:
			m[k] = v.GetIntValue()
		case *protoscommon.RichValue_Int64Value:
			m[k] = v.GetInt64Value()
		case *protoscommon.RichValue_BigintValue:
			m[k] = parseBigInt(v.GetBigintValue())
		case *protoscommon.RichValue_BigdecimalValue:
			m[k] = parseBigDecimal(v.GetBigdecimalValue())
		case *protoscommon.RichValue_FloatValue:
			m[k] = v.GetFloatValue()
		case *protoscommon.RichValue_BoolValue:
			m[k] = v.GetBoolValue()
		case *protoscommon.RichValue_BytesValue:
			m[k] = v.GetBytesValue()
		case *protoscommon.RichValue_TimestampValue:
			t := v.GetTimestampValue().AsTime().UTC()
			m[k] = fmt.Sprintf("toDateTime('%s', 'UTC')", t.Format("2006-01-02 15:04:05"))
		default:
			log.Warnf("unsupported rich value type: %T", v.Value)
			continue
		}
	}

	return FormatOption{
		OptID:               richStruct,
		richStructParameter: richStructParameter,
		parsedParameter:     m,
	}
}

type arg struct {
	k string
	v any
}

type args []arg

func (a args) Len() int {
	return len(a)
}

func (a args) Less(i, j int) bool {
	return len(a[i].k) > len(a[j].k)
}

func (a args) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func FormatSQLTemplate(sqlTemplate string, context map[string]any, opts ...FormatOption) string {
	opt := &FormatOption{}
	opt.Merge(opts...)

	prefixList, suffixList := opt.parameterPrefixList, opt.parameterSuffixList
	if len(prefixList) == 0 || len(suffixList) == 0 {
		prefixList, suffixList = []string{defaultPrefix}, []string{defaultSuffix}
	}

	var args args
	if opt.parsedParameter != nil {
		for k, v := range opt.parsedParameter {
			args = append(args, arg{k, v})
		}
	}
	if context != nil {
		for k, v := range context {
			args = append(args, arg{k, v})
		}
	}
	sort.Sort(args)
	var replaceArgs = make([]string, 0, len(args)*2)
	for _, a := range args {
		for i := range prefixList {
			replaceArgs = append(replaceArgs, prefixList[i]+a.k+suffixList[i], anyutil.ParseString(a.v))
		}
	}
	return strings.NewReplacer(replaceArgs...).Replace(sqlTemplate)
}
