(in-package :acc)

(fiveam:def-suite parser)

(test return-value-test
  (is (= 0 (parse-program (tokenize "func main int { return 0; }"))))
  (is (= 100 (parse-program (tokenize "func main int { return 100; }")))))