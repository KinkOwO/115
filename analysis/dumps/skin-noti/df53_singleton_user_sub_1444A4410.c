// singleton_user_sub_1444A4410

__int64 __fastcall sub_1444A4410(__int64 a1, __int64 *a2)
{
  __int64 *v2; // rdi
  int v4; // esi
  unsigned int v5; // r15d
  __int64 v6; // r14
  __int64 result; // rax
  __int64 v8; // r12
  __int64 *v9; // rcx
  __int64 v10; // rdx
  int v11; // eax
  __int64 v12; // rcx
  __int64 v13; // rax
  void (__fastcall ***v14)(_QWORD); // rcx
  __int64 v15; // rcx
  __int64 v16; // rax
  void (__fastcall ***v17)(_QWORD); // rcx
  unsigned int v18; // ebp
  void (__fastcall *v19)(__int64, __int64, __int64, _QWORD, int, int, int, _DWORD, _DWORD, int, int); // rsi
  double v20; // xmm0_8
  int v21; // edi
  unsigned int v22; // ebx
  __int64 v23; // rdx
  __int64 v24; // r8
  __int64 v25; // rdi
  void (__fastcall *v26)(__int64, __int64); // rbx
  __int64 v27; // rax
  __int64 v28; // rdi
  void (__fastcall *v29)(__int64, __int64); // rbx
  __int64 v30; // rax
  __int64 v31; // rdi
  void (__fastcall *v32)(__int64, __int64); // rbx
  __int64 v33; // rax
  __int64 v34; // rax
  __int64 v35; // rax
  __int64 v36; // rdi
  void (__fastcall *v37)(__int64, __int64); // rbx
  __int64 v38; // rax
  __int64 v39; // rax
  __int64 v40; // rax
  int v41; // [rsp+28h] [rbp-E0h]
  int v42; // [rsp+50h] [rbp-B8h]
  _BYTE v43[16]; // [rsp+78h] [rbp-90h] BYREF
  _BYTE v44[24]; // [rsp+88h] [rbp-80h] BYREF
  unsigned int v45; // [rsp+110h] [rbp+8h]
  unsigned int v47; // [rsp+120h] [rbp+18h]
  int v48; // [rsp+128h] [rbp+20h]

  v2 = a2;
  sub_1467A6790(a1, a2);
  v4 = sub_145F12D90(qword_14E683C20);
  v48 = v4;
  v5 = 0;
  v6 = 0;
  do
  {
    result = *v2;
    if ( *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 16) == *v2 )
    {
      v8 = sub_145F13700(qword_14E683C20, v5);
      v9 = *(__int64 **)(v6 + *(_QWORD *)(a1 + 1488) + 32);
      v10 = *v9;
      LOBYTE(v10) = v8 != 0;
      (*(void (__fastcall **)(__int64 *, __int64))(*v9 + 16))(v9, v10);
      sub_146EF7900(*(_QWORD *)(*(_QWORD *)(a1 + 1488) + v6 + 48), v8 != 0);
      sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 80), v8 != 0);
      result = sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 64), v8 != 0);
      if ( v8 )
      {
        v11 = dword_14F1C077C;
        if ( v4 == v5 )
          v11 = dword_14F1C0A0C;
        v47 = v11;
        v12 = qword_14E659EA8;
        if ( !qword_14E659EA8 )
        {
          v13 = sub_146E8BA20(496);
          if ( v13 )
            v14 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v13);
          else
            v14 = 0;
          qword_14E659EA8 = (__int64)v14;
          (**v14)(v14);
          v12 = qword_14E659EA8;
        }
        v45 = sub_14449E100(v12, v5);
        v15 = qword_14E659EA8;
        if ( !qword_14E659EA8 )
        {
          v16 = sub_146E8BA20(496);
          if ( v16 )
            v17 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v16);
          else
            v17 = 0;
          qword_14E659EA8 = (__int64)v17;
          (**v17)(v17);
          v15 = qword_14E659EA8;
        }
        v18 = sub_14449E0F0(v15, v5);
        v19 = *(void (__fastcall **)(__int64, __int64, __int64, _QWORD, int, int, int, _DWORD, _DWORD, int, int))(*(_QWORD *)v8 + 16LL);
        v20 = sub_146EC9470(*v2);
        v21 = (int)*(float *)&v20;
        v22 = (int)sub_146ECA0B0(*a2);
        sub_142757420(*a2);
        sub_146ECA0C0(*a2);
        LOBYTE(v42) = 0;
        LOBYTE(v41) = 0;
        v19(v8, v23, v24, v22, v21, v41, 255, 0, 0, 1065353216, v42);
        v25 = *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 32);
        v26 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v25 + 688LL);
        v27 = sub_141D37640(v8);
        v26(v25, v27);
        v28 = *(_QWORD *)(*(_QWORD *)(a1 + 1488) + v6 + 48);
        v29 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v28 + 688LL);
        v30 = sub_145F003E0(v8);
        v29(v28, v30);
        v31 = *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 80);
        v32 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v31 + 688LL);
        v33 = sub_14723C170(100001118);
        v34 = sub_146E8CF20(v43, v33, v45);
        v35 = sub_14014F430(v34);
        v32(v31, v35);
        sub_146E8C910(v43);
        v36 = *(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 64);
        v37 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v36 + 688LL);
        v38 = sub_14723C170(100001119);
        v39 = sub_146E8CF20(v44, v38, v18);
        v40 = sub_14014F430(v39);
        v37(v36, v40);
        sub_146E8C910(v44);
        sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 32), v47);
        sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 48), v47);
        sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 80), v47);
        result = sub_146EF7900(*(_QWORD *)(v6 + *(_QWORD *)(a1 + 1488) + 64), v47);
        v2 = a2;
        v4 = v48;
      }
    }
    ++v5;
    v6 += 96;
  }
  while ( (int)v5 < 8 );
  return result;
}

