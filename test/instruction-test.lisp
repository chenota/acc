(in-package :acc)

(fiveam:def-suite instruction)

(fiveam:test string-operand-test
  (fiveam:is (string= "\"VALUE\"" (to-string (make-string-operand "VALUE"))))
  (fiveam:signals error (make-string-operand 0)))

(fiveam:test ident-operand-test
  (fiveam:is (string= "VALUE" (to-string (make-ident-operand "VALUE"))))
  (fiveam:signals error (make-ident-operand 0)))

(fiveam:test type-operand-test
  (fiveam:is (string= "@VALUE" (to-string (make-type-operand "VALUE"))))
  (fiveam:signals error (make-type-operand 0)))

(fiveam:test immediate-operand-test
  (fiveam:is (string= "$0" (to-string (make-immediate-operand 0))))
  (fiveam:is (string= "$200" (to-string (make-immediate-operand 200))))
  (fiveam:signals error (make-immediate-operand "burger")))

(fiveam:test number-operand-test
  (fiveam:is (string= "0" (to-string (make-number-operand 0))))
  (fiveam:is (string= "200" (to-string (make-number-operand 200))))
  (fiveam:signals error (make-number-operand "burger")))

(fiveam:test gpreg32-operand-test
  (fiveam:is (string= "%eax" (to-string (make-gpreg32-operand 0))))
  (fiveam:is (string= "%r15d" (to-string (make-gpreg32-operand 15))))
  (fiveam:signals error (make-gpreg32-operand 1000))
  (fiveam:signals error (make-gpreg32-operand -1))
  (fiveam:signals error (make-gpreg32-operand "burger")))

(fiveam:test gpreg64-operand-test
  (fiveam:is (string= "%rax" (to-string (make-gpreg64-operand 0))))
  (fiveam:is (string= "%r15" (to-string (make-gpreg64-operand 15))))
  (fiveam:signals error (make-gpreg64-operand 1000))
  (fiveam:signals error (make-gpreg64-operand -1))
  (fiveam:signals error (make-gpreg64-operand "burger")))

(fiveam:def-fixture instruction-test-env ()
  (let ((generic-op (make-ident-operand "VALUE")))
    (&body)))

(fiveam:test instruction-test
  (fiveam:with-fixture instruction-test-env ()
    (fiveam:is (string=
          (format nil "~cop" #\tab)
          (to-string (make-instance 'instruction :op "op"))))
    (fiveam:is (string=
          (format nil "~cop VALUE" #\tab)
          (to-string (make-instance 'instruction :op "op" :oprs (list generic-op)))))
    (fiveam:is (string=
          (format nil "~cop VALUE, VALUE" #\tab)
          (to-string (make-instance 'instruction :op "op" :oprs (list generic-op generic-op)))))
    (fiveam:signals error (make-instruction "add" "some" :value 15))))

(fiveam:test label-test
  (fiveam:is (string= "main:" (to-string (make-label "main")))))