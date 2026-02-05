(in-package :acc/test)

(fiveam:def-suite instruction)
(fiveam:in-suite instruction)

(fiveam:test string-operand-test
  (fiveam:is (string= "\"VALUE\"" (acc::to-string (acc::make-string-operand "VALUE"))))
  (fiveam:signals error (acc::make-string-operand 0)))

(fiveam:test ident-operand-test
  (fiveam:is (string= "VALUE" (acc::to-string (acc::make-ident-operand "VALUE"))))
  (fiveam:signals error (acc::make-ident-operand 0)))

(fiveam:test type-operand-test
  (fiveam:is (string= "@VALUE" (acc::to-string (acc::make-type-operand "VALUE"))))
  (fiveam:signals error (acc::make-type-operand 0)))

(fiveam:test immediate-operand-test
  (fiveam:is (string= "$0" (acc::to-string (acc::make-immediate-operand 0))))
  (fiveam:is (string= "$200" (acc::to-string (acc::make-immediate-operand 200))))
  (fiveam:signals error (acc::make-immediate-operand "burger")))

(fiveam:test number-operand-test
  (fiveam:is (string= "0" (acc::to-string (acc::make-number-operand 0))))
  (fiveam:is (string= "200" (acc::to-string (acc::make-number-operand 200))))
  (fiveam:signals error (acc::make-number-operand "burger")))

(fiveam:test gpreg32-operand-test
  (fiveam:is (string= "%eax" (acc::to-string (acc::make-gpreg32-operand 0))))
  (fiveam:is (string= "%r15d" (acc::to-string (acc::make-gpreg32-operand 15))))
  (fiveam:signals error (acc::make-gpreg32-operand 1000))
  (fiveam:signals error (acc::make-gpreg32-operand -1))
  (fiveam:signals error (acc::make-gpreg32-operand "burger")))

(fiveam:test gpreg64-operand-test
  (fiveam:is (string= "%rax" (acc::to-string (acc::make-gpreg64-operand 0))))
  (fiveam:is (string= "%r15" (acc::to-string (acc::make-gpreg64-operand 15))))
  (fiveam:signals error (acc::make-gpreg64-operand 1000))
  (fiveam:signals error (acc::make-gpreg64-operand -1))
  (fiveam:signals error (acc::make-gpreg64-operand "burger")))

(fiveam:def-fixture instruction-test-env ()
  (let ((generic-op (acc::make-ident-operand "VALUE")))
    (&body)))

(fiveam:test instruction-test
  (fiveam:with-fixture instruction-test-env ()
    (fiveam:is (string=
          (format nil "~cop" #\tab)
          (acc::to-string (make-instance 'acc::instruction :op "op"))))
    (fiveam:is (string=
          (format nil "~cop VALUE" #\tab)
          (acc::to-string (make-instance 'acc::instruction :op "op" :oprs (list generic-op)))))
    (fiveam:is (string=
          (format nil "~cop VALUE, VALUE" #\tab)
          (acc::to-string (make-instance 'acc::instruction :op "op" :oprs (list generic-op generic-op)))))
    (fiveam:signals error (acc::make-instruction "add" "some" :value 15))))

(fiveam:test label-test
  (fiveam:is (string= "main:" (acc::to-string (acc::make-label "main")))))