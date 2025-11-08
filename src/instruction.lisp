(in-package :acc)

(defclass operand ()
    ())

(defmethod :before print-operand ((o operand) s)
  (check-type s stream))

(defmethod print-operand ((o operand) s)
  (format s "UNDEFINED"))

(defmethod to-string ((o operand))
  (with-output-to-string (s)
    (print-operand o s)))

(defclass instruction ()
    ((operation :initarg :op :type string :accessor instr-op)
     (operands :initarg :oprs :type sequence :accessor instr-oprs :initform nil)
     (indent :initarg :indent :type boolean :accessor instr-indent :initform t)))

(defmethod print-instruction ((i instruction) s)
  (check-type s stream)
  (when (instr-indent i) (format s "~c" #\Tab))
  (format s "~a" (instr-op i))
  (let ((opr-count (length (instr-oprs i))))
    (when (> opr-count 0) (write-string " " s))
    (loop
   for operand in (instr-oprs i)
   for j = 0 then (1+ j)
   do (print-operand operand s)
     when (< j (1- opr-count)) do (write-string ", " s))))

(defmethod to-string ((i instruction))
  (with-output-to-string (s)
    (print-instruction i s)))