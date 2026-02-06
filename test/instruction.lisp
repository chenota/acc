(in-package :acc/test)

(fiveam:def-suite instruction)
(fiveam:in-suite instruction)

(fiveam:test string-operand-success (fiveam:is (string= "\"VALUE\"" (acc::to-string (acc::make-string-operand "VALUE")))))
(fiveam:test string-operand-failure (fiveam:signals error (acc::make-string-operand 0)))

(fiveam:test ident-operand-success (fiveam:is (string= "VALUE" (acc::to-string (acc::make-ident-operand "VALUE")))))
(fiveam:test ident-operand-failure (fiveam:signals error (acc::make-ident-operand 0)))

(fiveam:test type-oprand-success (fiveam:is (string= "@VALUE" (acc::to-string (acc::make-type-operand "VALUE")))))
(fiveam:test type-operand-failure (fiveam:signals error (acc::make-type-operand 0)))

(fiveam:test immediate-operand-success (fiveam:is (string= "$60" (acc::to-string (acc::make-immediate-operand 60)))))
(fiveam:test immediate-operand-failure (fiveam:signals error (acc::make-immediate-operand "burger")))

(fiveam:test number-operand-success (fiveam:is (string= "10" (acc::to-string (acc::make-number-operand 10)))))
(fiveam:test number-operand-failure (fiveam:signals error (acc::make-number-operand "burger")))

(defmacro test-reg-operands (size name0 name15)
  (let ((constructor (intern (format nil "MAKE-GPREG~A-OPERAND" size) :acc)))
    `(progn
      (fiveam:test ,(intern (format nil "GPREG-~A-LOWER" size)) (fiveam:is (string= ,name0 (acc::to-string (,constructor 0)))))
      (fiveam:test ,(intern (format nil "GPREG-~A-UPPER" size)) (fiveam:is (string= ,name15 (acc::to-string (,constructor 15)))))
      (fiveam:test ,(intern (format nil "GPREG-~A-FAILURE" size)) (fiveam:signals error (,constructor "burger")))
      (fiveam:test ,(intern (format nil "GPREG-~A-RANGE-LOW" size)) (fiveam:signals error (,constructor -1)))
      (fiveam:test ,(intern (format nil "GPREG-~A-RANGE-HIGH" size)) (fiveam:signals error (,constructor 16))))))

(test-reg-operands 8 "%al" "%r15b")
(test-reg-operands 16 "%ax" "%r15w")
(test-reg-operands 32 "%eax" "%r15d")
(test-reg-operands 64 "%rax" "%r15")

(fiveam:test instruction-no-args (fiveam:is (string= "op" (trimmed-string-from (make-instance 'acc::instruction :op "op")))))
(fiveam:test instruction-one-arg (fiveam:is (string= "op V" (trimmed-string-from (make-instance 'acc::instruction :op "op" :oprs (list (acc::make-ident-operand "V")))))))
(fiveam:test instruction-two-args (fiveam:is (string= "op V, V" (trimmed-string-from (make-instance 'acc::instruction :op "op" :oprs (list (acc::make-ident-operand "V") (acc::make-ident-operand "V")))))))

(fiveam:test label (fiveam:is (string= "main:" (trimmed-string-from (acc::make-label "main")))))