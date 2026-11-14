package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteVariables(t *testing.T) {
	data := map[string]interface{}{"javaversion": "17", "port": 25565, "quote": `a"b'c\d`}
	tests := []struct {
		condition, want string
	}{
		{`in_path("java${javaversion}")`, `in_path("java17")`},
		{`in_path('java${javaversion}')`, `in_path('java17')`},
		{`in_path("""java${javaversion}""")`, `in_path("""java17""")`},
		{`in_path(r"java${javaversion}")`, `in_path(r"java" + "17" + r"")`},
		{`${port} > 1000`, `port > 1000`},
		{`"${quote}"`, `"a\"b\'c\\d"`},
		{`"${missing}"`, `"${missing}"`},
		{`b"${javaversion}"`, `b"${javaversion}"`},
		{`"\"${javaversion}"`, `"\"17"`},
		{`"${os}"`, `"` + GlobalConstantValues[VariableOs].(string) + `"`},
		{`success`, `success`},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, SubstituteVariables(tt.condition, data), tt.condition)
	}
}

func TestResolveIf_SubstitutesVariables(t *testing.T) {
	data := map[string]interface{}{"javaversion": "17", "port": 25565}

	got, err := ResolveIf(`"java${javaversion}" == "java17"`, data, nil)
	require.NoError(t, err)
	assert.True(t, got)

	got, err = ResolveIf(`r"java${javaversion}" == "java17"`, data, nil)
	require.NoError(t, err)
	assert.True(t, got)

	got, err = ResolveIf(`${port} > 1000`, data, nil)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestResolveIf_AValueCannotBecomeCode(t *testing.T) {
	// a variable the user sets must stay text inside the literal, not end it
	for _, value := range []string{`x" || true || "`, `x' || true || '`, `x\" || true || \"`} {
		data := map[string]interface{}{"name": value}
		for _, condition := range []string{`"${name}" == "safe"`, `'${name}' == 'safe'`, `r"${name}" == "safe"`} {
			got, err := ResolveIf(condition, data, nil)
			require.NoError(t, err, "%s with %s", condition, value)
			assert.False(t, got, "%s with %s", condition, value)
		}

		got, err := ResolveIf(`"${name}" == name`, data, nil)
		require.NoError(t, err)
		assert.True(t, got, "the value arrives unchanged: %s", value)
	}
}
