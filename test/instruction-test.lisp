(in-package :acc)

(fiveam:def-suite instruction)

(test string-operand-test
  (is (string= "\"VALUE\"" (to-string (make-string-operand "VALUE"))))
  (signals error (make-string-operand 0)))

(test ident-operand-test
  (is (string= "VALUE" (to-string (make-ident-operand "VALUE"))))
  (signals error (make-ident-operand 0)))

(test type-operand-test
  (is (string= "@VALUE" (to-string (make-type-operand "VALUE"))))
  (signals error (make-type-operand 0)))

(test immediate-operand-test
  (is (string= "$0" (to-string (make-immediate-operand 0))))
  (is (string= "$200" (to-string (make-immediate-operand 200))))
  (signals error (make-immediate-operand "burger")))

(test gpreg32-operand-test
  (is (string= "%eax" (to-string (make-gpreg32-operand 0))))
  (is (string= "%r15d" (to-string (make-gpreg32-operand 15))))
  (signals error (make-gpreg32-operand 1000))
  (signals error (make-gpreg32-operand -1))
  (signals error (make-gpreg32-operand "burger")))

(def-fixture instruction-test-env ()
  (let ((generic-op (make-ident-operand "VALUE")))
    (&body)))

(test instruction-test
  (with-fixture instruction-test-env ()
    (is (string=
          (format nil "~cop" #\tab)
          (to-string (make-instance 'instruction :op "op"))))
    (is (string=
          (format nil "~cop VALUE" #\tab)
          (to-string (make-instance 'instruction :op "op" :oprs (list generic-op)))))
    (is (string=
          (format nil "~cop VALUE, VALUE" #\tab)
          (to-string (make-instance 'instruction :op "op" :oprs (list generic-op generic-op)))))
    (signals error (make-instruction "add" "some" :value 15))))

(test label-test
  (is (string= "main:" (to-string (make-label "main")))))