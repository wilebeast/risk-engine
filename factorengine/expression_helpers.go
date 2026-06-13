package factorengine

func isNumericType(t ValueType) bool {
	return t == ValueTypeInt || t == ValueTypeLong || t == ValueTypeDouble
}

func mergeNumericType(left, right ValueType) ValueType {
	if left == ValueTypeDouble || right == ValueTypeDouble {
		return ValueTypeDouble
	}
	if left == ValueTypeLong || right == ValueTypeLong {
		return ValueTypeLong
	}
	return ValueTypeInt
}

func areComparableOrderedTypes(left, right ValueType) bool {
	if isNumericType(left) && isNumericType(right) {
		return true
	}
	return left == ValueTypeString && right == ValueTypeString
}

func areEqualityCompatible(left, right ValueType) bool {
	if left == ValueTypeNull || right == ValueTypeNull {
		return true
	}
	if left == right {
		return true
	}
	return isNumericType(left) && isNumericType(right)
}

func mergeComparableType(left, right ValueType) ValueType {
	if left == ValueTypeNull {
		return right
	}
	if right == ValueTypeNull {
		return left
	}
	if left == right {
		return left
	}
	if isNumericType(left) && isNumericType(right) {
		return mergeNumericType(left, right)
	}
	return left
}

func areBranchCompatible(left, right ValueType) bool {
	if left == ValueTypeNull || right == ValueTypeNull {
		return true
	}
	return areEqualityCompatible(left, right)
}
