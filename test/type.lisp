(in-package :acc/test)

(fiveam:def-suite type)
(fiveam:in-suite type)

(defun int-type-from (str)
  (fiveam:finishes (acc::integer-type-size (type-from str))))

(fiveam:test atomic-type-int8 (fiveam:is (eq :int8 (int-type-from "int8"))))

(fiveam:test atomic-type-int16 (fiveam:is (eq :int16 (int-type-from "int16"))))

(fiveam:test atomic-type-int32 (fiveam:is (eq :int32 (int-type-from "int32"))))

(fiveam:test atomic-type-int (fiveam:is (eq :int32 (int-type-from "int"))))

(fiveam:test atomic-type-int64 (fiveam:is (eq :int64 (int-type-from "int64"))))

(fiveam:test type-int-error (fiveam:signals error (type-from "100")))