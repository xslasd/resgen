# scalar_test.res
module ScalarCenter

scalar IntTime: int64

input ScalarInput {
    time: IntTime @query
    body_time: IntTime
}

type ComplexA {
    name: String!
    scalarVal: IntTime!
}

type ComplexB {
    title: String!
    aSingle: ComplexA!
    aPointer: ComplexA
    aArrayVal: [ComplexA!]!
    aArrayPtr: [ComplexA]
}

type ScalarOutput {
    time: IntTime
    nestedComplex: ComplexB
}

group ScalarGroup /scalar {
    GET /test/:time => TestScalar (time: IntTime @path) : ResData<ScalarOutput>
    POST /test => TestScalarBody (input: ScalarInput) : ResData<ScalarOutput>
}
