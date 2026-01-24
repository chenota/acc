(in-package :acc)

(defun assign-types (ast function-return-context)
  (cond
   ((program-node-p ast)
     (loop for func in (program-node-functions ast) do (assign-types func function-return-context)))
   ((function-node-p ast)
     (loop for stmt in (function-node-body ast) do (assign-types stmt (function-node-return-type ast)))
     (setf (function-node-type-info ast) (make-function-type :parameters nil :return-type (function-node-return-type ast))))
   ((return-statement-node-p ast)
     (assign-types (return-statement-node-expression ast) function-return-context)
     (unless
         (valid-cast-p (ast-node-type-info (return-statement-node-expression ast)) function-return-context)
       (error 'location-error
         :location (return-statement-node-location ast)
         :message (format nil "Invalid return type: ~A" (ast-node-type-info (return-statement-node-expression ast))))))
   ((cast-node-p ast)
     (assign-types (cast-node-expression ast) function-return-context)
     (unless (valid-cast-p (ast-node-type-info (cast-node-expression ast)) (cast-node-cast-type ast))
       (error
           'location-error
         :location (cast-node-location ast)
         :message (format nil "Invalid type cast: ~A to ~A" (ast-node-type-info (cast-node-expression ast)) (cast-node-cast-type ast))))
     (setf (cast-node-type-info ast) (cast-node-cast-type ast)))
   ((int-node-p ast)
     (setf (int-node-type-info ast) (make-primitive-type :kind :untyped-int)))))

(defun valid-cast-p (source-type destination-type)
  (declare (ignore source-type))
  ;; Only invalid cast is to a nil type
  (not (null destination-type)))