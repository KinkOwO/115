// sub_140B939B0  va=0x140B939B0  size=398

char __fastcall sub_140B939B0(void *a1, int a2)
{
  __int64 v4; // rbx
  __int64 v5; // rax
  __int64 v6; // rax
  void *v7; // rax
  void *v8; // rdi
  __int64 v9; // rax
  int v10; // ebp
  _QWORD *v11; // r14
  __int64 v12; // rbx
  __int64 v14; // [rsp+20h] [rbp-78h]
  __int64 v15; // [rsp+20h] [rbp-78h]
  __int64 v16; // [rsp+20h] [rbp-78h]
  void *v17; // [rsp+30h] [rbp-68h]
  _BYTE v18[16]; // [rsp+38h] [rbp-60h] BYREF
  __int64 v19; // [rsp+48h] [rbp-50h]
  _BYTE v20[32]; // [rsp+50h] [rbp-48h] BYREF
  void *v21; // [rsp+70h] [rbp-28h]

  v21 = a1;
  __eh34_enter_wind_state(-1, 0);
  v4 = qword_14E6399A0;
  if ( qword_14E6399A0 == 0 )
  {
    v5 = sub_146E8BA20(1360);
    v14 = v5;
    __wind
    {
      if ( v5 != 0 )
        v4 = sub_140B8D280(v5);
      else
        v4 = 0;
    }
    __unwind
    {
      j_j_scalable_free(v14, 1360);
    }
    qword_14E6399A0 = v4;
  }
  if ( a2 < *(_DWORD *)(v4 + 208) )
  {
    if ( v4 == 0 )
    {
      v6 = sub_146E8BA20(1360);
      v15 = v6;
      __wind
      {
        if ( v6 != 0 )
          v4 = sub_140B8D280(v6);
        else
          v4 = 0;
      }
      __unwind
      {
        j_j_scalable_free(v15, 1360);
      }
      qword_14E6399A0 = v4;
    }
    sub_14014C810(v20, a1);
    v8 = v7;
    v17 = v7;
    __eh34_enter_wind_state(0, 4);
    v9 = qword_14E6399A0;
    if ( qword_14E6399A0 == 0 )
    {
      v9 = sub_146E8BA20(1360);
      v16 = v9;
      __wind
      {
        if ( v9 != 0 )
          v9 = sub_140B8D280(v9);
      }
      __unwind
      {
        j_j_scalable_free(v16, 1360);
      }
      qword_14E6399A0 = v9;
    }
    v10 = *(_DWORD *)(v9 + 236);
    if ( v10 != 0 )
    {
      if ( v10 < 0
        || (v11 = (_QWORD *)(v4 + 952), sub_140150980(v4 + 952, v18, v8), v12 = v19, *(_BYTE *)(v19 + 25) != 0)
        || (unsigned __int8)sub_140151510(v11, v8, v19 + 32) != 0
        || v12 == *v11
        || *(_DWORD *)(v12 + 64) < v10 )
      {
        if ( __eh34_unwind(4) )
          goto unwind_state_4;
        __eh34_exit_wind_state(4, 0);
        unknown_libname_4(v8);
        if ( __eh34_unwind(0) )
          goto unwind_state_0;
        __eh34_exit_wind_state(0, -1);
        unknown_libname_4(a1);
        return 1;
      }
    }
    if ( __eh34_unwind(4) )
    {
unwind_state_4:
      unknown_libname_4(v17);
      __eh34_continue_unwinding(4, 0);
    }
    __eh34_exit_wind_state(4, 0);
    unknown_libname_4(v8);
  }
  if ( __eh34_unwind(0) )
  {
unwind_state_0:
    unknown_libname_4(v21);
    __eh34_propagate_exception_into_caller(0, -1);
  }
  __eh34_exit_wind_state(0, -1);
  unknown_libname_4(a1);
  return 0;
}
